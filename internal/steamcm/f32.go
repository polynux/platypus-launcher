package steamcm

import "math"

func float32FromBits(u uint32) float32 { return math.Float32frombits(u) }

func bitsOf(f float32) uint32 { return math.Float32bits(f) }
