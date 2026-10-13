package types

// SaveGate is what a backup-before-delete step did before a destructive
// action (issue #42): which backup covers the deleted saves. #43 defines
// this minimal shape so the removal job can call the gate; #42 keeps these
// fields and may add more.
type SaveGate struct {
	// BackupID names the backup that covers the deleted saves ("" when none
	// was written).
	BackupID string `json:"backup_id,omitempty"`
	// Files and Bytes count what the backup holds.
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
	// NoPaths means no known save paths applied to the deleted folders: the
	// UI asks for an explicit "delete without a save backup" confirmation.
	NoPaths bool `json:"no_paths"`
}
