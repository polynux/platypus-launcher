//go:build windows

package main

import (
	"os"
)

// Windows handle duplication and inherited-handle cleanup are not implemented yet.
func protocolFile() (*os.File, error) { return os.Stdout, nil }
func closeInheritedFds()              {}
func shutdownSignals() []os.Signal    { return []os.Signal{os.Interrupt} }
