package id

import (
	"testing"

	"github.com/google/uuid"
)

func TestNew_ValidUUIDv7(t *testing.T) {
	got := New()
	parsed, err := uuid.Parse(got)
	if err != nil {
		t.Fatalf("New() returned unparseable UUID %q: %v", got, err)
	}
	if v := parsed.Version(); v != 7 {
		t.Fatalf("expected v7, got v%d (%q)", v, got)
	}
}

func TestNew_MonotonicWithinMillisecond(t *testing.T) {
	// UUIDv7 must be strictly increasing when sorted lexicographically,
	// even across rapid successive calls within the same millisecond.
	const n = 1000
	prev := New()
	for i := 0; i < n; i++ {
		cur := New()
		if cur <= prev {
			t.Fatalf("UUIDv7 not monotonic: prev=%s cur=%s", prev, cur)
		}
		prev = cur
	}
}
