// Package id generates IDs for single-node OwnForge deployments.
//
// UUIDv7 is the default: 128 bits, time-ordered (monotonic within a
// millisecond), no coordination required, safe to generate from many
// processes. This replaces the standalone id-generator service for
// local/self-hosted installs.
package id

import "github.com/google/uuid"

// New returns a new UUIDv7 as a canonical string.
// It panics if the system entropy source fails, matching the behavior
// callers expect from a pure ID helper.
func New() string {
	return uuid.Must(uuid.NewV7()).String()
}

// NewBytes returns a new UUIDv7 as its 16-byte binary form.
func NewBytes() [16]byte {
	return uuid.Must(uuid.NewV7())
}
