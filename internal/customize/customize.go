// Package customize keeps the user's own look for a game: a title, a sort
// title and the images of its page and library entry (cover, banner, logo, icon,
// background). It is core-only: nothing here touches a plugin's data. Images
// are copied into Platypus's data folder, so the original file can move or go,
// and are checked first (a known image type, a size cap). Resetting a part
// brings the store's own art back.
package customize

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // registered for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Kind names one image of a game. The set is closed: an unknown kind is refused.
type Kind string

const (
	Cover      Kind = "cover"      // portrait art of library cards
	Banner     Kind = "banner"     // wide art: the game page's hero and library banners
	Logo       Kind = "logo"       // the title's logo, drawn over the hero
	Icon       Kind = "icon"       // the small square next to the title in lists
	Background Kind = "background" // faint backdrop of the game page
)

// Kinds lists every kind, in the order the UI shows them.
var Kinds = []Kind{Cover, Banner, Logo, Icon, Background}

// Valid says whether k is one of Kinds.
func (k Kind) Valid() bool {
	for _, x := range Kinds {
		if k == x {
			return true
		}
	}
	return false
}

const (
	// MaxImageBytes caps one image file. Art is a few hundred KB; this leaves
	// room for a large PNG without letting a stray file fill the data folder.
	MaxImageBytes = 15 << 20
	// MaxImageSide caps either dimension, so a small file can't decode into
	// gigabytes when the UI draws it.
	MaxImageSide = 10000
	// MaxTitleLen caps a custom title or sort title, in characters.
	MaxTitleLen = 200

	// URLPrefix is where the asset server serves the images (see Handler).
	URLPrefix = "/platypus-art/"
)

// Entry is what the user changed about one game.
type Entry struct {
	Title     string         `json:"title,omitempty"`
	SortTitle string         `json:"sort_title,omitempty"`
	Art       map[Kind]Image `json:"art,omitempty"`
	// ArtSource remembers where the art came from when Platypus found it
	// online (issue #36), e.g. "steam:12345", with ArtSourceName the store
	// name ("Hollow Knight") for the attribution line. Empty means the art
	// is the store's or the user's own.
	ArtSource     string `json:"art_source,omitempty"`
	ArtSourceName string `json:"art_source_name,omitempty"`
}

// Image is one stored image: its file name inside the game's folder and a
// version that changes with every replacement, so the UI never shows a cached
// older picture.
type Image struct {
	File    string `json:"file"`
	Version int64  `json:"v"`
}

// empty says the entry holds nothing.
func (e Entry) empty() bool { return e.Title == "" && e.SortTitle == "" && len(e.Art) == 0 }

type file struct {
	Version int              `json:"version"`
	Games   map[string]Entry `json:"games,omitempty"`
}

// Store persists the customizations under one folder: customization.json and
// art/<key>/<kind>.<ext>. The zero value is not usable; call Open.
type Store struct {
	dir  string
	mu   sync.Mutex
	data file
}

// Open loads the store at dir (created on first write). A missing index gives
// an empty store; a corrupt one is moved aside (customization.json.bad-<time>)
// and Open returns an empty store along with the error.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, data: file{Version: 1}}
	raw, err := os.ReadFile(s.indexPath())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", s.indexPath(), time.Now().Unix())
		_ = os.Rename(s.indexPath(), bad)
		s.data = file{Version: 1}
		return s, fmt.Errorf("customization file is corrupt, moved to %s: %w", bad, err)
	}
	return s, nil
}

func (s *Store) indexPath() string { return filepath.Join(s.dir, "customization.json") }
func (s *Store) artRoot() string   { return filepath.Join(s.dir, "art") }

// key is the folder name of a game's images: a hash, so no game id can name a
// path.
func key(gameID string) string {
	sum := sha256.Sum256([]byte(gameID))
	return hex.EncodeToString(sum[:8])
}

func (s *Store) gameDir(gameID string) string { return filepath.Join(s.artRoot(), key(gameID)) }

// Get returns the customization of a game (the zero Entry when it has none).
// The returned value shares nothing with the store.
func (s *Store) Get(gameID string) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.data.Games[gameID])
}

func clone(e Entry) Entry {
	if e.Art != nil {
		art := make(map[Kind]Image, len(e.Art))
		for k, v := range e.Art {
			art[k] = v
		}
		e.Art = art
	}
	return e
}

// URL is the address the UI draws kind of gameID from ("" when it has none).
func (s *Store) URL(gameID string, kind Kind) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.urlLocked(gameID, kind)
}

func (s *Store) urlLocked(gameID string, kind Kind) string {
	img, ok := s.data.Games[gameID].Art[kind]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s%s/%s?v=%d", URLPrefix, key(gameID), url.PathEscape(img.File), img.Version)
}

// Path is the stored file of kind ("" when there is none).
func (s *Store) Path(gameID string, kind Kind) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	img, ok := s.data.Games[gameID].Art[kind]
	if !ok {
		return ""
	}
	return filepath.Join(s.gameDir(gameID), img.File)
}

// Snapshot is a read-only view of every customization, for decorating a whole
// library without taking the lock per game.
type Snapshot struct {
	games map[string]Entry
	urls  map[string]map[Kind]string
}

// Snapshot copies the current state.
func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{games: make(map[string]Entry, len(s.data.Games)), urls: map[string]map[Kind]string{}}
	for id, e := range s.data.Games {
		snap.games[id] = clone(e)
		for kind := range e.Art {
			if snap.urls[id] == nil {
				snap.urls[id] = map[Kind]string{}
			}
			snap.urls[id][kind] = s.urlLocked(id, kind)
		}
	}
	return snap
}

// Entry returns the customization of a game in the snapshot.
func (sn Snapshot) Entry(gameID string) Entry { return sn.games[gameID] }

// URL returns the address of one image in the snapshot.
func (sn Snapshot) URL(gameID string, kind Kind) string { return sn.urls[gameID][kind] }

// cleanTitle trims a title and refuses control characters and a long one.
func cleanTitle(what, t string) (string, error) {
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > MaxTitleLen {
		return "", fmt.Errorf("the %s is longer than %d characters", what, MaxTitleLen)
	}
	for _, r := range t {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("the %s has a control character", what)
		}
	}
	return t, nil
}

// SetTitle sets the custom title and sort title of a game. An empty one
// clears that part (the store's title, or the sort by title).
func (s *Store) SetTitle(gameID, title, sortTitle string) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	title, err := cleanTitle("title", title)
	if err != nil {
		return err
	}
	sortTitle, err = cleanTitle("sort title", sortTitle)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateLocked(gameID, func(e *Entry) error {
		e.Title, e.SortTitle = title, sortTitle
		return nil
	})
}

// updateLocked applies change to a copy of the game's entry and saves; the
// in-memory state only changes when the save worked.
func (s *Store) updateLocked(gameID string, change func(*Entry) error) error {
	prev, had := s.data.Games[gameID]
	e := clone(prev)
	if err := change(&e); err != nil {
		return err
	}
	if s.data.Games == nil {
		s.data.Games = map[string]Entry{}
	}
	if e.empty() {
		delete(s.data.Games, gameID)
	} else {
		s.data.Games[gameID] = e
	}
	if err := s.saveLocked(); err != nil {
		if had {
			s.data.Games[gameID] = prev
		} else {
			delete(s.data.Games, gameID)
		}
		return err
	}
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "customization-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(raw)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), s.indexPath()); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// imageExt names the image type of the first bytes of a file, or "" for a
// type Platypus doesn't take. The type comes from the content, never from the
// file's name; SVG is refused (it can carry script).
func imageExt(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case bytes.HasPrefix(head, []byte("\xff\xd8\xff")):
		return ".jpg"
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		return ".gif"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return ".webp"
	}
	return ""
}

// CheckImage validates a file as art: a regular file, no larger than
// MaxImageBytes, of a known image type, with sane dimensions. It returns the
// extension to store it under.
func CheckImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("can't read the image: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("can't read the image: %w", err)
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("choose an image file")
	}
	if st.Size() == 0 {
		return "", errors.New("the image file is empty")
	}
	if st.Size() > MaxImageBytes {
		return "", fmt.Errorf("the image is %.1f MB; the limit is %d MB", float64(st.Size())/1e6, MaxImageBytes>>20)
	}
	head := make([]byte, 32)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("can't read the image: %w", err)
	}
	ext := imageExt(head[:n])
	if ext == "" {
		return "", errors.New("that is not a PNG, JPEG, GIF or WebP image")
	}
	if ext != ".webp" { // the standard library decodes the header of the others
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return "", fmt.Errorf("the image is damaged: %w", err)
		}
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxImageSide || cfg.Height > MaxImageSide {
			return "", fmt.Errorf("the image is %d×%d; the limit is %d pixels on a side", cfg.Width, cfg.Height, MaxImageSide)
		}
	}
	return ext, nil
}

// SetArt copies the image at src into the store as kind of gameID, replacing
// the one it had. The source file is checked first and never modified. A
// hand-picked cover, banner or logo ends the online attribution (see
// SetArtSource): the visible art is the user's now.
func (s *Store) SetArt(gameID string, kind Kind, src string) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	if !kind.Valid() {
		return fmt.Errorf("unknown kind of art %q", kind)
	}
	ext, err := CheckImage(src)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.gameDir(gameID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := string(kind) + ext
	if err := copyInto(dir, name, src); err != nil {
		return err
	}
	prevImg, had := s.data.Games[gameID].Art[kind]
	err = s.updateLocked(gameID, func(e *Entry) error {
		if e.Art == nil {
			e.Art = map[Kind]Image{}
		}
		e.Art[kind] = Image{File: name, Version: time.Now().UnixNano()}
		if kind == Cover || kind == Banner || kind == Logo {
			e.ArtSource, e.ArtSourceName = "", ""
		}
		return nil
	})
	if err != nil {
		if !had || prevImg.File != name {
			_ = os.Remove(filepath.Join(dir, name))
		}
		return err
	}
	if had && prevImg.File != name { // the old file had another extension
		_ = os.Remove(filepath.Join(dir, prevImg.File))
	}
	return nil
}

// copyInto copies src to dir/name through a temporary file, so a failed copy
// never leaves half an image under the final name. The source is checked
// again on the open file's size: it may have grown since CheckImage.
func copyInto(dir, name, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, ".copy-*")
	if err != nil {
		return err
	}
	n, err := io.Copy(tmp, io.LimitReader(in, MaxImageBytes+1))
	cerr := tmp.Close()
	if err = errors.Join(err, cerr); err == nil && n > MaxImageBytes {
		err = fmt.Errorf("the image is larger than %d MB", MaxImageBytes>>20)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// SetArtSource remembers where the game's art came from (issue #36), e.g.
// source "steam:12345" with the store name for the attribution line. It is
// only set after an online apply stored art; setting it with no custom art
// is refused.
func (s *Store) SetArtSource(gameID, source, name string) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updateLocked(gameID, func(e *Entry) error {
		if len(e.Art) == 0 {
			return errors.New("no custom art to attribute")
		}
		e.ArtSource, e.ArtSourceName = source, name
		return nil
	})
}

// ResetArt drops the custom image of kind: the store's art shows again.
// Dropping the last of the cover, banner and logo also drops the online
// attribution (issue #36): there is no looked-up art left to credit.
func (s *Store) ResetArt(gameID string, kind Kind) error {
	if !kind.Valid() {
		return fmt.Errorf("unknown kind of art %q", kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	img, ok := s.data.Games[gameID].Art[kind]
	if !ok {
		return nil
	}
	err := s.updateLocked(gameID, func(e *Entry) error {
		delete(e.Art, kind)
		if len(e.Art) == 0 {
			e.Art = nil
		}
		if e.ArtSource != "" && !hasOnlineArt(e) {
			e.ArtSource, e.ArtSourceName = "", ""
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(s.gameDir(gameID), img.File))
	return nil
}

// hasOnlineArt says whether the entry keeps looked-up art worth crediting:
// a custom cover, banner or logo.
func hasOnlineArt(e *Entry) bool {
	for _, k := range []Kind{Cover, Banner, Logo} {
		if _, ok := e.Art[k]; ok {
			return true
		}
	}
	return false
}

// Reset drops everything the user changed about a game, files included.
func (s *Store) Reset(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.Games[gameID]; !ok {
		return nil
	}
	prev := s.data.Games[gameID]
	delete(s.data.Games, gameID)
	if err := s.saveLocked(); err != nil {
		s.data.Games[gameID] = prev
		return err
	}
	_ = os.RemoveAll(s.gameDir(gameID))
	return nil
}
