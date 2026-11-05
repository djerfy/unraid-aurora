package provider

import (
	"math"
	"testing"
)

func TestSafeUnsignedConversions(t *testing.T) {
	if got := safeUint64(-1); got != 0 {
		t.Fatalf("safeUint64(-1) = %d", got)
	}
	if got := safeUint64Int(-1); got != 0 {
		t.Fatalf("safeUint64Int(-1) = %d", got)
	}
	if got := safeUint64(42); got != 42 {
		t.Fatalf("safeUint64(42) = %d", got)
	}
	if got := safeUint32(-1); got != 0 {
		t.Fatalf("safeUint32(-1) = %d", got)
	}
	if got := safeUint32(math.MaxInt); got != math.MaxUint32 {
		t.Fatalf("safeUint32 overflow = %d", got)
	}
}
