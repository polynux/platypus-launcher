package savebackup

import (
	"context"
	"fmt"
	"net/http"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"platypus/internal/settings"
	"platypus/internal/types"
)

// EventKind names the backup event channel messages ("backups").
type EventKind string

const (
	EvStarted  EventKind = "started"
	EvDone     EventKind = "done"
	EvFailed   EventKind = "failed"
	EvManifest EventKind = "manifest"
)

// Event is one message on the backups channel. It carries ids and counts,
// never paths.
type Event struct {
	Kind     EventKind `json:"kind"`
	GameID   string    `json:"game_id,omitempty"`
	BackupID string    `json:"backup_id,omitempty"`
	Trigger  string    `json:"trigger,omitempty"`
	Files    int       `json:"files,omitempty"`
	Bytes    int64     `json:"bytes,omitempty"`
	Warned   bool      `json:"warned,omitempty"`
	Message  string    `json:"message,omitempty"`
}

// GameLookup resolves a game id to its game (the App passes this so the
// service stays free of the library).
type GameLookup func(gameID string) (types.Game, bool)

// Service orchestrates manifest refreshes, backups and restores.
type Service struct {
	settings *settings.Store
	dataDir  string
	cacheDir string
	home     string

	manifestURL string
	client      *http.Client
	emit        func(Event)

	lookup GameLookup

	mu       sync.Mutex
	fetching bool
	now      func() time.Time
}

// Options configures a Service.
type Options struct {
	Settings    *settings.Store
	DataDir     string
	CacheDir    string
	Home        string
	ManifestURL string
	Client      *http.Client
	Emit        func(Event)
	Lookup      GameLookup
}

// New makes a Service. It cleans stale .tmp files at startup.
func New(o Options) *Service {
	url := o.ManifestURL
	if url == "" {
		url = DefaultManifestURL
	}
	s := &Service{
		settings: o.Settings, dataDir: o.DataDir, cacheDir: o.CacheDir, home: o.Home,
		manifestURL: url, client: o.Client, emit: o.Emit, lookup: o.Lookup,
		now: time.Now,
	}
	CleanStaleTmp(s.backupsRoot())
	return s
}

func (s *Service) backupsRoot() string {
	if s.settings != nil {
		return s.settings.ResolvedBackupsDir(s.dataDir)
	}
	return filepath.Join(s.dataDir, "backups")
}

func (s *Service) emitEv(e Event) {
	if s.emit != nil {
		s.emit(e)
	}
}

// ManifestStateV reports the visible manifest state.
func (s *Service) ManifestStateV() string {
	s.mu.Lock()
	fetching := s.fetching
	s.mu.Unlock()
	if fetching {
		return "fetching"
	}
	return ManifestState(s.cacheDir)
}

// MaybeRefresh refreshes the manifest in the background when it is older
// than 7 days. It never blocks the caller.
func (s *Service) MaybeRefresh(ctx context.Context) {
	if ManifestFetchedAt(s.cacheDir) != 0 && time.Since(time.Unix(ManifestFetchedAt(s.cacheDir), 0)) < ManifestRefreshInterval {
		return
	}
	if LoadIndex(s.cacheDir) == nil {
		// No cache at all: still refresh, games fall back to user paths meanwhile.
	}
	go s.Refresh(ctx)
}

// Refresh fetches the manifest now (the "Update save list" button). A 304
// keeps the cache; a 200 rebuilds the index. Concurrent refreshes coalesce.
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	if s.fetching {
		s.mu.Unlock()
		return nil
	}
	s.fetching = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.fetching = false
		s.mu.Unlock()
	}()
	etag := StoredEtag(s.cacheDir)
	changed, newEtag, data, err := FetchManifest(s.cacheDir, s.manifestURL, etag, s.client)
	if err != nil {
		s.emitEv(Event{Kind: EvManifest, Message: err.Error()})
		return err
	}
	if !changed {
		// Touch the meta so the next MaybeRefresh waits another interval.
		StoreManifestTouch(s.cacheDir, etag)
		s.emitEv(Event{Kind: EvManifest, Message: "up to date"})
		return nil
	}
	if _, err := StoreManifest(s.cacheDir, data, newEtag); err != nil {
		s.emitEv(Event{Kind: EvManifest, Message: err.Error()})
		return err
	}
	s.emitEv(Event{Kind: EvManifest, Message: "updated"})
	return nil
}

// StoreManifestTouch refreshes the fetch timestamp without new data (304).
func StoreManifestTouch(cacheDir, etag string) {
	meta := manifestMeta{Etag: etag, FetchedAt: time.Now().Unix()}
	_ = os.MkdirAll(savesCacheDir(cacheDir), 0o755)
	b, _ := json.Marshal(meta)
	_ = os.WriteFile(metaPath(cacheDir), b, 0o644)
}

// GameIDs splits a game's store identity: the compat Steam AppID (or the
// store id for store games) and the gog id from info_source "gog:<id>".
func GameIDs(game types.Game, compatAppID, infoSource string) (steamID, gogID string) {
	if game.Source != "" && game.Source != types.SourceCustom {
		if isSteamSource(game) {
			steamID = game.StoreAppID
		} else if isGogSource(game) {
			gogID = game.StoreAppID
		} else {
			// Plugin games keyed by Steam or GOG id still match by id.
			steamID = game.StoreAppID
		}
	}
	if compatAppID != "" {
		steamID = compatAppID
	}
	if strings.HasPrefix(infoSource, "gog:") {
		gogID = strings.TrimPrefix(infoSource, "gog:")
	}
	return steamID, gogID
}

func isSteamSource(game types.Game) bool {
	return game.Source == types.SourceSteam || game.Store == types.StoreSteam
}

func isGogSource(game types.Game) bool {
	return game.Store == types.StoreGOG
}

// ResolveFor resolves a game's save locations: user paths first, then the
// manifest match. steamID/gogID are the game's store ids (see GameIDs);
// runnerPath/prefixesFolder/steamRoot tune the context.
func (s *Service) ResolveFor(game types.Game, steamID, gogID, runnerPath, prefixesFolder, steamRoot string, userPaths []string) ([]types.SavePath, []resolvedPattern, *GameEntry, Context, int) {
	ctx := NewContextFor(game, s.home, prefixesFolder, s.dataDir, runnerPath, steamRoot)
	ctx.SteamID, ctx.GogID = steamID, gogID
	var entry *GameEntry
	if idx := LoadIndex(s.cacheDir); idx != nil {
		entry = Match(idx, game.OriginalTitle(), steamID, gogID)
	}
	roots, patterns, unresolved := ResolvePaths(game, userPaths, entry, ctx, steamID, gogID)
	return roots, patterns, entry, ctx, unresolved
}

// BackupOptions tunes one backup run.
type BackupOptions struct {
	RunnerPath     string
	PrefixesFolder string
	SteamRoot      string
	UserPaths      []string
	SteamID        string
	GogID          string
	ManifestTag    string // ETag override for tests; "" uses the stored one
	Now            time.Time
}

// Backup backs up a game's saves under trigger. It returns the backup (with
// Unchanged set when the saves equal the newest backup and nothing was
// written), whether the 1 GiB warning tripped, and the skipped/unresolved
// counts. Above 10 GiB it refuses.
func (s *Service) Backup(game types.Game, trigger string, o BackupOptions) (*types.SaveBackup, bool, int, int, error) {
	now := o.Now
	if now.IsZero() {
		now = s.now()
	}
	ctx := NewContextFor(game, s.home, o.PrefixesFolder, s.dataDir, o.RunnerPath, o.SteamRoot)
	ctx.SteamID, ctx.GogID = o.SteamID, o.GogID
	var entry *GameEntry
	if idx := LoadIndex(s.cacheDir); idx != nil {
		entry = Match(idx, game.OriginalTitle(), o.SteamID, o.GogID)
	}
	_, patterns, unresolved := ResolvePaths(game, o.UserPaths, entry, ctx, o.SteamID, o.GogID)
	files, skipped, _, _, bytes := Enumerate(patterns, ctx)
	tag := o.ManifestTag
	if tag == "" {
		tag = StoredEtag(s.cacheDir)
		if tag == "" {
			tag = "user"
		}
	}
	// Dedupe: path+size+mtime equal the newest backup → return it unchanged.
	if newest := newestBackup(s.backupsRoot(), game.ID); newest != nil {
		if meta, ok := ReadBackupMeta(s.backupsRoot(), game.ID, newest.ID); ok && DedupeEqual(files, meta) {
			cp := *newest
			cp.Unchanged = true
			return &cp, bytes > WarnBytes, skipped, unresolved, nil
		}
	}
	rootAbs := map[string]string{}
	for label, root := range ctx.Roots {
		rootAbs[label] = root
	}
	b, total, err := WriteBackup(s.backupsRoot(), game, trigger, files, rootAbs, tag, unresolved, skipped, now)
	if err != nil {
		s.emitEv(Event{Kind: EvFailed, GameID: game.ID, Trigger: trigger, Message: err.Error()})
		return nil, false, skipped, unresolved, err
	}
	warned := total > WarnBytes
	keep := DefaultKeep(s.settings)
	PruneOldest(s.backupsRoot(), game.ID, keep)
	s.emitEv(Event{Kind: EvDone, GameID: game.ID, BackupID: b.ID, Trigger: trigger, Files: b.Files, Bytes: b.Bytes, Warned: warned})
	return b, warned, skipped, unresolved, nil
}

// DefaultKeep is the versions kept per game from settings (default 10).
func DefaultKeep(st *settings.Store) int {
	if st == nil {
		return 10
	}
	return st.KeepBackups()
}

func newestBackup(root, gameID string) *types.SaveBackup {
	list := ListBackups(root, gameID)
	if len(list) == 0 {
		return nil
	}
	b := list[0]
	return &b
}

// List lists a game's backups, newest first.
func (s *Service) List(gameID string) []types.SaveBackup {
	return ListBackups(s.backupsRoot(), gameID)
}

// Delete removes one backup version. The id is the zip's base name, with or
// without the .zip suffix.
func (s *Service) Delete(gameID, backupID string) error {
	id := strings.TrimSuffix(backupID, ".zip")
	if id == "" || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("unknown backup %q", backupID)
	}
	p := filepath.Join(gameBackupDir(s.backupsRoot(), gameID), id+".zip")
	if _, _, ok := readMeta(p); !ok {
		return fmt.Errorf("unknown backup %q", backupID)
	}
	return os.Remove(p)
}

// RestorePreview previews a restore.
func (s *Service) RestorePreview(gameID, backupID string, ctx Context) (types.RestorePreview, error) {
	pv, _, err := PreviewRestore(s.backupsRoot(), gameID, backupID, ctx)
	return pv, err
}

// Restore restores a backup, after taking a before-restore backup first. If
// the safety backup fails, the restore is blocked.
func (s *Service) Restore(game types.Game, backupID string, ctx Context, safety func() error) (int, int, error) {
	if safety != nil {
		if err := safety(); err != nil {
			return 0, 0, fmt.Errorf("before-restore backup failed, restore blocked: %w", err)
		}
	}
	return Restore(s.backupsRoot(), game.ID, backupID, ctx)
}

// BackupsApplies says whether backups apply to a game: the global mode is
// not off and the per-game override is not off.
func BackupsApplies(st *settings.Store, gameID string) bool {
	if st == nil {
		return true
	}
	if st.BackupsMode() == settings.BackupsOff {
		return false
	}
	return st.GameSaveBackup(gameID) != "off"
}

var _ = context.Background
