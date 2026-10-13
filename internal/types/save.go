package types

// SaveGate is the preview a destructive action shows before it deletes
// anything (issue #42). BackupID names the backup the gate just wrote (""
// when none); Files and Bytes count what it holds; NoPaths means Platypus
// knows no save paths for the game, so the UI must ask for an explicit
// "delete without a save backup" confirmation.
type SaveGate struct {
	BackupID string `json:"backup_id,omitempty"`
	Files    int    `json:"files"`
	Bytes    int64  `json:"bytes"`
	NoPaths  bool   `json:"no_paths"`
	// Unchanged is true when the saves equal the newest backup, so the gate
	// wrote nothing and reused it.
	Unchanged bool `json:"unchanged,omitempty"`
	// Off is true in a preview when backups don't apply to the game (turned
	// off globally or for it): nothing will be backed up first.
	Off bool `json:"off,omitempty"`
}

// SavePath is one resolved save location of a game: where it is on disk,
// where it came from, and whether it exists right now.
type SavePath struct {
	Path   string `json:"path"`
	Origin string `json:"origin"` // "user" | "manifest"
	Exists bool   `json:"exists"`
}

// SaveFile is one file inside a backup archive.
type SaveFile struct {
	Root   string `json:"root"` // root label from the metadata
	Path   string `json:"path"` // relative path inside that root
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256,omitempty"`
	Mtime  int64  `json:"mtime,omitempty"`
}

// SaveBackup is one backup version of a game.
type SaveBackup struct {
	ID        string `json:"id"`
	GameID    string `json:"game_id"`
	CreatedAt int64  `json:"created_at"` // unix seconds
	Trigger   string `json:"trigger"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Size      int64  `json:"size"` // zip size on disk
	Unchanged bool   `json:"unchanged,omitempty"`
}

// BackupMeta is the platypus-backup.json written last inside every zip.
type BackupMeta struct {
	Version    int               `json:"version"`
	GameID     string            `json:"game_id"`
	Title      string            `json:"title"`
	Source     string            `json:"source"`
	CreatedAt  int64             `json:"created_at"`
	Trigger    string            `json:"trigger"`
	Manifest   string            `json:"manifest"` // manifest ETag or "user"
	Roots      map[string]string `json:"roots"`
	Files      []SaveFile        `json:"files"`
	Unresolved int               `json:"unresolved"`
	Skipped    int               `json:"skipped"`
	Bytes      int64             `json:"bytes"`
}

// BackupSettings are the global save-backup switches (issue #42).
type BackupSettings struct {
	Folder        string `json:"folder"`         // backups folder, "" = <data>/backups
	DefaultFolder string `json:"default_folder"` // <data>/backups
	Keep          int    `json:"keep"`           // versions kept per game
	Mode          string `json:"mode"`           // "destructive" | "all" | "off"
	AfterSession  bool   `json:"after_session"`
	Manifest      string `json:"manifest"` // "ready" | "fetching" | "stale" | "none"
}

// RestorePreview says what a restore would do, before it does it.
type RestorePreview struct {
	Overwritten int   `json:"overwritten"`
	Added       int   `json:"added"`
	Kept        int   `json:"kept"`
	Files       int   `json:"files"`
	Bytes       int64 `json:"bytes"`
}
