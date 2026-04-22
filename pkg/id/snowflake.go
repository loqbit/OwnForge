package id

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	snowflakeEpochMillis int64 = 1704067200000 // 2024-01-01T00:00:00Z
	nodeBits             int64 = 10
	sequenceBits         int64 = 12
	maxNodeID            int64 = (1 << nodeBits) - 1
	maxSequence          int64 = (1 << sequenceBits) - 1
	nodeShift                  = sequenceBits
	timestampShift             = nodeBits + sequenceBits
)

// Generator is the minimal ID-generation interface used by services.
type Generator interface {
	NextID(ctx context.Context) (int64, error)
	Close() error
}

// LocalSnowflake generates int64 IDs in-process using timestamp + node_id + sequence.
type LocalSnowflake struct {
	mu            sync.Mutex
	nodeID        int64
	lastTimestamp int64
	sequence      int64
	now           func() time.Time
}

// NewLocalSnowflake creates a process-local snowflake generator.
func NewLocalSnowflake(nodeID int64) (*LocalSnowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("node_id %d out of range [0,%d]", nodeID, maxNodeID)
	}

	return &LocalSnowflake{
		nodeID: nodeID,
		now:    time.Now,
	}, nil
}

// NextID returns the next int64 snowflake ID.
func (g *LocalSnowflake) NextID(ctx context.Context) (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ts := g.now().UnixMilli()
	if ts < g.lastTimestamp {
		var err error
		ts, err = g.waitUntil(ctx, g.lastTimestamp)
		if err != nil {
			return 0, err
		}
	}

	if ts == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			var err error
			ts, err = g.waitUntil(ctx, g.lastTimestamp+1)
			if err != nil {
				return 0, err
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = ts

	delta := ts - snowflakeEpochMillis
	if delta < 0 {
		return 0, fmt.Errorf("system clock predates snowflake epoch: %d", ts)
	}

	id := (delta << timestampShift) | (g.nodeID << nodeShift) | g.sequence
	return id, nil
}

func (g *LocalSnowflake) waitUntil(ctx context.Context, targetMillis int64) (int64, error) {
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		nowMillis := g.now().UnixMilli()
		if nowMillis >= targetMillis {
			return nowMillis, nil
		}

		time.Sleep(time.Millisecond)
	}
}

// Close exists to satisfy the shared generator interface.
func (g *LocalSnowflake) Close() error {
	return nil
}
