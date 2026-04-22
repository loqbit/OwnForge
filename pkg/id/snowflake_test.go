package id

import (
	"context"
	"testing"
)

func TestNewLocalSnowflake_RejectsInvalidNodeID(t *testing.T) {
	t.Parallel()

	if _, err := NewLocalSnowflake(-1); err == nil {
		t.Fatal("expected negative node_id to fail")
	}
	if _, err := NewLocalSnowflake(maxNodeID + 1); err == nil {
		t.Fatal("expected oversized node_id to fail")
	}
}

func TestLocalSnowflakeNextID_UniqueAndIncreasing(t *testing.T) {
	t.Parallel()

	gen, err := NewLocalSnowflake(7)
	if err != nil {
		t.Fatalf("NewLocalSnowflake() error = %v", err)
	}

	const n = 5000
	prev, err := gen.NextID(context.Background())
	if err != nil {
		t.Fatalf("first NextID() error = %v", err)
	}

	seen := map[int64]struct{}{prev: {}}
	for i := 1; i < n; i++ {
		cur, err := gen.NextID(context.Background())
		if err != nil {
			t.Fatalf("NextID() error at %d = %v", i, err)
		}
		if cur <= prev {
			t.Fatalf("ids must be strictly increasing: prev=%d cur=%d", prev, cur)
		}
		if _, ok := seen[cur]; ok {
			t.Fatalf("duplicate id generated: %d", cur)
		}
		seen[cur] = struct{}{}
		prev = cur
	}
}
