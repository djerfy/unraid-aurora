package provider

import "math"

// safeUint64 keeps host-reported sizes usable even when a malformed INI value
// or a filesystem reports a negative number.
func safeUint64(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func safeUint64Int(value int) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func safeUint32(value int) uint32 {
	if value <= 0 {
		return 0
	}
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value)
}
