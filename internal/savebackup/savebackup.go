// Package savebackup backs up game save files for every game (issue #42).
//
// Save locations come from the user's own paths first, then Ludusavi's open
// manifest (a YAML file of save paths per game, keyed by title and Steam id).
// Backups are one zip per run under <dataDir>/backups (or the user's folder),
// with a platypus-backup.json manifest inside. Restores write only the files
// the backup holds and always take a before-restore backup first.
package savebackup

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// DefaultManifestURL is Ludusavi's open manifest of save paths per game.
const DefaultManifestURL = "https://raw.githubusercontent.com/mtkennerly/ludusavi-manifest/master/data/manifest.yaml"

// ManifestRefreshInterval is how often the manifest is refreshed in the
// background (at most every 7 days), plus the manual "Update save list".
const ManifestRefreshInterval = 7 * 24 * time.Hour

// IndexVersion is the derived index format version.
const IndexVersion = 2 // 2: when: store constraints (issue #60)

// BackupMetaVersion is the platypus-backup.json format version.
const BackupMetaVersion = 1

// WarnBytes warns above 1 GiB of saves; RefuseBytes refuses above 10 GiB
// ("narrow the paths"). Constants, per the design.
const (
	WarnBytes   = int64(1) << 30
	RefuseBytes = int64(10) << 30
)

// shortID makes the random tail of a backup file name.
func shortID() string {
	var b [4]byte
	// crypto/rand never fails in practice; fall back to time on error.
	if _, err := randRead(b[:]); err != nil {
		return time.Now().Format("0405")
	}
	return hex.EncodeToString(b[:])
}

func randRead(b []byte) (int, error) {
	return randReader.Read(b)
}

var randReader = cryptoRand{}

type cryptoRand struct{}

func (cryptoRand) Read(b []byte) (int, error) {
	// Small indirection so tests can replace the reader without importing
	// crypto/rand in every file.
	return crRead(b)
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
