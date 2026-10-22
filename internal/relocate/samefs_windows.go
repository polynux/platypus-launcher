//go:build windows

package relocate

// Windows same-volume detection is not implemented; copy safely across volumes.
func sameDevice(string, string) (bool, error) { return false, nil }
