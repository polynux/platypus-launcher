// Job state and installed manifests: resume, updates and verify support.
// State lives in HYDRA_PLUGIN_DATA/jobs/<job>.json (manifest ids and
// completed chunk ids, flushed as chunks land); the installed manifest per
// depot in HYDRA_PLUGIN_DATA/manifests/<app>/<depot>_<gid>.manifest.
package depot

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// JobState tracks one download job for resume.
type JobState struct {
	AppID      uint32          `json:"app_id"`
	DepotID    uint32          `json:"depot_id"`
	ManifestID uint64          `json:"manifest_id"`
	Branch     string          `json:"branch,omitempty"`
	Done       map[string]bool `json:"done"` // hex chunk id -> fetched
	Total      int             `json:"total"`
}

// SaveJob writes the state atomically (temp file + rename).
func SaveJob(dataDir, jobID string, st *JobState) error {
	dir := filepath.Join(dataDir, "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.Marshal(st)
	tmp := filepath.Join(dir, jobID+".json.new")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, jobID+".json"))
}

// LoadJob reads a job state, or nil when there is none (fresh start).
func LoadJob(dataDir, jobID string) (*JobState, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, "jobs", jobID+".json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st JobState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	if st.Done == nil {
		st.Done = map[string]bool{}
	}
	return &st, nil
}

// RemoveJob drops a finished job's state.
func RemoveJob(dataDir, jobID string) error {
	err := os.Remove(filepath.Join(dataDir, "jobs", jobID+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SaveManifest stores an installed manifest's raw bytes.
func SaveManifest(dataDir string, appID, depotID uint32, gid uint64, raw []byte) error {
	dir := filepath.Join(dataDir, "manifests", itoa(appID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, manifestName(depotID, gid)+".new")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, manifestName(depotID, gid)))
}

// LoadManifest reads an installed manifest, or nil when absent.
func LoadManifest(dataDir string, appID, depotID uint32, gid uint64) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, "manifests", itoa(appID), manifestName(depotID, gid)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}

func manifestName(depotID uint32, gid uint64) string {
	return itoa(depotID) + "_" + itoa64(gid) + ".manifest"
}

func hexID(id []byte) string { return hex.EncodeToString(id) }

func itoa(n uint32) string { return strconv.FormatUint(uint64(n), 10) }

func itoa64(n uint64) string { return strconv.FormatUint(n, 10) }
