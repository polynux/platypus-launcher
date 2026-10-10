package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxJSON bounds the files read for the rewrite.
const maxJSON = 64 << 20

// skipDirs are never entered: Wine prefixes are moved as folders (their
// registries are not edited), plugins hold code, logs hold text.
var skipDirs = map[string]bool{"prefixes": true, "plugins": true, "plugins-before-rename": true, "logs": true}

// RewritePaths rewrites, in every JSON file under root that Platypus or a
// plugin persisted, each string that names a place inside the old data
// folder oldRoot so it names the same place inside root. That covers the
// settings (wine prefixes, install folders, relocation records), the job
// queue, the install ledger, pending cloud pushes, the customization index,
// the compat caches and the plugins' own state, without knowing their shapes:
// every string value and every key is looked at. It returns the changed files,
// relative to root, and is idempotent. Files are replaced atomically with
// their permissions kept (tokens are 0600).
func RewritePaths(root, oldRoot string, warn func(string, ...any)) ([]string, error) {
	r := newRewriter(oldRoot, root)
	var changed []string
	var firstErr error
	walk, err := filepath.EvalSymlinks(root) // the data folder may itself be a link
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(walk, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if filepath.Dir(path) == walk && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		did, err := r.file(path)
		if err != nil {
			// One unreadable or unparsable file must not hide the others;
			// but a file we could not write is a real failure.
			rel, _ := filepath.Rel(walk, path)
			if warn != nil {
				warn("migration: leaving %s as it is: %v", rel, err)
			}
			var we *writeError
			if errors.As(err, &we) && firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if did {
			rel, _ := filepath.Rel(walk, path)
			changed = append(changed, rel)
		}
		return nil
	})
	if err == nil {
		err = firstErr
	}
	return changed, err
}

type writeError struct{ err error }

func (e *writeError) Error() string { return e.err.Error() }
func (e *writeError) Unwrap() error { return e.err }

type rewriter struct {
	olds []string // spellings of the old root
	new  string
}

func newRewriter(oldRoot, newRoot string) *rewriter {
	olds := []string{oldRoot}
	rel := ".local/share/" + legacyName
	for _, h := range []string{"~", "$HOME", "${HOME}"} {
		olds = append(olds, h+"/"+rel)
	}
	return &rewriter{olds: olds, new: newRoot}
}

func isPathChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("._-/+@%~", c) >= 0
}

// str rewrites one string. An occurrence counts only as a whole path prefix:
// not glued to a longer name before ("/mnt" + old root) or after (the old
// folder's name followed by "-extra").
func (r *rewriter) str(s string) (string, bool) {
	any := false
	for _, old := range r.olds {
		var c bool
		if s, c = replacePrefix(s, old, r.new); c {
			any = true
		}
	}
	return s, any
}

func replacePrefix(s, old, new string) (string, bool) {
	var b strings.Builder
	changed := false
	pos := 0
	for {
		i := strings.Index(s[pos:], old)
		if i < 0 {
			break
		}
		start := pos + i
		end := start + len(old)
		b.WriteString(s[pos:start])
		okBefore := start == 0 || !isPathChar(s[start-1])
		okAfter := end == len(s) || s[end] == '/' || !isPathChar(s[end])
		if okBefore && okAfter {
			b.WriteString(new)
			changed = true
		} else {
			b.WriteString(old)
		}
		pos = end
	}
	if !changed {
		return s, false
	}
	b.WriteString(s[pos:])
	return b.String(), true
}

func (r *rewriter) value(v any) (any, bool) {
	switch t := v.(type) {
	case string:
		return r.str(t)
	case []any:
		any2 := false
		for i := range t {
			nv, c := r.value(t[i])
			t[i], any2 = nv, any2 || c
		}
		return t, any2
	case map[string]any:
		any2 := false
		for _, k := range keysOf(t) {
			nv, c := r.value(t[k])
			nk, kc := r.str(k)
			if c || kc {
				any2 = true
				delete(t, k)
				t[nk] = nv
			}
		}
		return t, any2
	}
	return v, false
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// file rewrites one JSON file; it reports whether it changed.
func (r *rewriter) file(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if fi.Size() > maxJSON {
		return false, errors.New("too big to rewrite")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	// Cheap exit: files that don't mention the old folder at all.
	if !bytes.Contains(raw, []byte(legacyName)) {
		return false, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return false, err
	}
	if dec.More() {
		return false, errors.New("more than one JSON value")
	}
	doc, changed := r.value(doc)
	if !changed {
		return false, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if bytes.IndexByte(bytes.TrimSpace(raw), '\n') >= 0 {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(doc); err != nil {
		return false, err
	}
	out := buf.Bytes()
	if !bytes.HasSuffix(raw, []byte("\n")) {
		out = bytes.TrimSuffix(out, []byte("\n"))
	}
	if err := replaceFile(path, out, fi.Mode().Perm()); err != nil {
		return false, &writeError{err: err}
	}
	return true, nil
}

// replaceFile writes data next to path and renames it over, so a crash leaves
// either the old or the new file whole.
func replaceFile(path string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rewrite-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(name)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}
