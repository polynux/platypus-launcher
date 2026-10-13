package savebackup

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileEntry is one file pattern of a manifest game entry that holds saves.
type FileEntry struct {
	Pattern   string   `json:"pattern"`
	Tags      []string `json:"tags,omitempty"`
	WhenOS    []string `json:"when_os,omitempty"`
	WhenStore []string `json:"when_store,omitempty"`
}

// GameEntry is one manifest game with save files: its titles, ids and file
// entries. Only entries with files: reach the derived index.
type GameEntry struct {
	Title   string      `json:"title"`
	Aliases []string    `json:"aliases,omitempty"`
	SteamID string      `json:"steam_id,omitempty"`
	GogID   string      `json:"gog_id,omitempty"`
	Files   []FileEntry `json:"files"`
}

// Index is the compact derived manifest read day to day: only entries with
// files:, holding title, aliases, steam/gog ids and the file entries.
type Index struct {
	Version int         `json:"version"`
	Etag    string      `json:"etag,omitempty"`
	BuiltAt int64       `json:"built_at"`
	Games   []GameEntry `json:"games"`
}

// dirs.go: cache layout helpers live here to keep manifest.go small.
func savesCacheDir(cacheDir string) string { return filepath.Join(cacheDir, "saves") }
func manifestPath(cacheDir string) string {
	return filepath.Join(savesCacheDir(cacheDir), "manifest.yaml")
}
func etagPath(cacheDir string) string { return filepath.Join(savesCacheDir(cacheDir), "manifest.etag") }
func metaPath(cacheDir string) string { return filepath.Join(savesCacheDir(cacheDir), "manifest.json") }
func indexPath(cacheDir string) string {
	return filepath.Join(savesCacheDir(cacheDir), "index.json.gz")
}

// manifestMeta remembers when the manifest was last fetched and its ETag.
type manifestMeta struct {
	Etag      string `json:"etag"`
	FetchedAt int64  `json:"fetched_at"`
}

// FetchManifest fetches the manifest with ETag / If-None-Match into the cache
// (<cacheDir>/saves/). A 304 keeps the cache; a 200 replaces it and rebuilds
// the derived index. It returns true when the cache changed. The only network
// call save backups ever make; it sends no identifiers.
func FetchManifest(cacheDir, url, etag string, client *http.Client) (changed bool, newEtag string, data []byte, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, etag, nil, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, etag, nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return false, etag, nil, nil
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		if err != nil {
			return false, etag, nil, err
		}
		return true, resp.Header.Get("ETag"), body, nil
	default:
		return false, etag, nil, fmt.Errorf("manifest: unexpected status %s", resp.Status)
	}
}

// StoredEtag reads the cached manifest ETag, "" when none.
func StoredEtag(cacheDir string) string {
	b, err := os.ReadFile(etagPath(cacheDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// StoreManifest writes the fetched manifest, its ETag and fetch time, and
// rebuilds the derived index.
func StoreManifest(cacheDir string, data []byte, etag string) (*Index, error) {
	if err := os.MkdirAll(savesCacheDir(cacheDir), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath(cacheDir), data, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(etagPath(cacheDir), []byte(etag), 0o644); err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(manifestMeta{Etag: etag, FetchedAt: time.Now().Unix()})
	_ = os.WriteFile(metaPath(cacheDir), meta, 0o644)
	return BuildIndex(data, etag, cacheDir)
}

// ManifestState reports the visible manifest state: ready (fresh index),
// fetching (a fetch is in flight), stale (index older than 7 days or a fetch
// failed) or none (no cache). fetching is set by the caller while a fetch
// runs; this function never reports it.
func ManifestState(cacheDir string) string {
	fi, err := os.Stat(indexPath(cacheDir))
	if err != nil {
		return "none"
	}
	if time.Since(fi.ModTime()) > ManifestRefreshInterval {
		return "stale"
	}
	return "ready"
}

// ManifestFetchedAt returns when the manifest was last fetched, 0 when never.
func ManifestFetchedAt(cacheDir string) int64 {
	b, err := os.ReadFile(metaPath(cacheDir))
	if err != nil {
		if fi, serr := os.Stat(indexPath(cacheDir)); serr == nil {
			return fi.ModTime().Unix()
		}
		return 0
	}
	var m manifestMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return 0
	}
	return m.FetchedAt
}

// LoadIndex reads the derived index, or nil when there is none.
func LoadIndex(cacheDir string) *Index {
	f, err := os.Open(indexPath(cacheDir))
	if err != nil {
		return nil
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil
	}
	defer gz.Close()
	var idx Index
	if err := json.NewDecoder(gz).Decode(&idx); err != nil {
		return nil
	}
	if idx.Version != IndexVersion {
		// An index from an older format: rebuild it from the manifest already
		// on disk (no download). Without a manifest there is no index yet.
		return rebuildIndex(cacheDir)
	}
	return &idx
}

// rebuildIndex derives a fresh index from the cached manifest, or nil.
func rebuildIndex(cacheDir string) *Index {
	data, err := os.ReadFile(manifestPath(cacheDir))
	if err != nil {
		return nil
	}
	idx, err := BuildIndex(data, StoredEtag(cacheDir), cacheDir)
	if err != nil {
		return nil
	}
	return idx
}

// BuildIndex parses the manifest YAML into the compact derived index and
// writes index.json.gz. It never decodes the whole YAML at once: the file is
// split on column-0 top-level keys and each chunk is parsed with yaml.v3.
func BuildIndex(data []byte, etag, cacheDir string) (*Index, error) {
	games, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	idx := &Index{Version: IndexVersion, Etag: etag, BuiltAt: time.Now().Unix(), Games: games}
	if err := WriteIndex(cacheDir, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// WriteIndex writes the derived index to index.json.gz.
func WriteIndex(cacheDir string, idx *Index) error {
	if err := os.MkdirAll(savesCacheDir(cacheDir), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(savesCacheDir(cacheDir), "index-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	gz := gzip.NewWriter(tmp)
	encErr := json.NewEncoder(gz).Encode(idx)
	cErr := gz.Close()
	if encErr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return encErr
	}
	if cErr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return cErr
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, indexPath(cacheDir))
}

// SplitManifest splits raw manifest YAML on column-0 top-level keys: each
// returned chunk is one "title:\n  ..." game block. Lines before the first
// key (comments) are dropped.
func SplitManifest(data []byte) [][]byte {
	var chunks [][]byte
	var cur []byte
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		// A top-level key starts at column 0, is not a comment or a list
		// item, and contains a colon.
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' && line[0] != '-' && bytes.IndexByte(line, ':') >= 0 {
			if cur != nil {
				chunks = append(chunks, cur)
			}
			cur = append([]byte(nil), line...)
			cur = append(cur, '\n')
			continue
		}
		if cur != nil {
			cur = append(cur, line...)
			cur = append(cur, '\n')
		}
	}
	if cur != nil {
		chunks = append(chunks, cur)
	}
	return chunks
}

// ParseManifest parses manifest YAML into game entries with save files.
func ParseManifest(data []byte) ([]GameEntry, error) {
	var out []GameEntry
	for _, chunk := range SplitManifest(data) {
		var m map[string]any
		dec := yaml.NewDecoder(bytes.NewReader(chunk))
		if err := dec.Decode(&m); err != nil {
			continue // a bad chunk never breaks the whole index
		}
		for title, raw := range m {
			g, ok := parseGameEntry(title, raw)
			if ok {
				out = append(out, g)
			}
		}
	}
	return out, nil
}

func parseGameEntry(title string, raw any) (GameEntry, bool) {
	body, ok := raw.(map[string]any)
	if !ok {
		return GameEntry{}, false
	}
	g := GameEntry{Title: title}
	if al, ok := body["aliases"]; ok {
		switch v := al.(type) {
		case string:
			g.Aliases = []string{v}
		case []any:
			for _, a := range v {
				if s, ok := a.(string); ok {
					g.Aliases = append(g.Aliases, s)
				}
			}
		}
	}
	if st, ok := body["steam"].(map[string]any); ok {
		g.SteamID = strID(st["id"])
	}
	if goG, ok := body["gog"].(map[string]any); ok {
		g.GogID = strID(goG["id"])
	}
	files, ok := body["files"].(map[string]any)
	if !ok {
		return GameEntry{}, false
	}
	for pattern, fraw := range files {
		fe := FileEntry{Pattern: pattern}
		fm, ok := fraw.(map[string]any)
		if !ok {
			continue
		}
		// tags: only save-tagged entries are backed up.
		tags := strList(fm["tags"])
		if len(tags) > 0 && !containsFold(tags, "save") {
			continue
		}
		fe.Tags = tags
		if w, ok := fm["when"].(map[string]any); ok {
			fe.WhenOS = strList(w["os"])
			fe.WhenStore = whenStores(w)
		} else if wl, ok := fm["when"].([]any); ok {
			for _, item := range wl {
				if wm, ok := item.(map[string]any); ok {
					fe.WhenOS = append(fe.WhenOS, strList(wm["os"])...)
					fe.WhenStore = append(fe.WhenStore, whenStores(wm)...)
				}
			}
		}
		g.Files = append(g.Files, fe)
	}
	if len(g.Files) == 0 {
		return GameEntry{}, false
	}
	return g, true
}

func strID(v any) string {
	switch t := v.(type) {
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%d", int64(t))
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func strList(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			} else if s := strID(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}

func appendOne(a, b any) any {
	la, lb := strList(a), strList(b)
	out := append(la, lb...)
	if len(out) == 0 {
		return nil
	}
	return out
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// whenStores collects a when: entry's store constraints. The manifest schema
// names the key "store" (e.g. "- store: steam", issue #60); entries pinning a
// store id directly ("steam"/"gog") are kept too.
func whenStores(w map[string]any) []string {
	var out []string
	out = append(out, strList(w["store"])...)
	out = append(out, strList(appendOne(w["steam"], w["gog"]))...)
	return out
}

// IndexHash is a short content hash for tests and logs (never paths).
func IndexHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}
