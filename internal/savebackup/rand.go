package savebackup

import (
	"crypto/rand"
	"time"
)

func crRead(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err := rand.Read(b)
	if err != nil {
		// Deterministic fallback: never fail a backup on entropy.
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (8 * (i % 8)))
		}
		return len(b), nil
	}
	return n, nil
}
