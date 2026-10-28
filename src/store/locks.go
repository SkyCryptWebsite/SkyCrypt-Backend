package store

import (
	"context"
	"encoding/binary"
)

// profileLockSpace is the first key of every per-profile advisory lock ("SK"), apart from migrationLock.
const profileLockSpace = 0x534b

// withProfileLock runs fn while holding a Postgres advisory lock for the profile on a connection of its own. Every
// change to a profile's value records and its board entries (ingest and backfill in the parent, hiding a player in
// a child) runs under it, so each reads, merges, writes and moves the boards as one step and none can push older
// values over newer ones.
func withProfileLock(ctx context.Context, profileID [16]byte, fn func() error) error {
	conn, err := postgres().Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Distinct profiles may share a key; that only makes them wait for each other.
	key := int32(binary.BigEndian.Uint32(profileID[:4]) ^ binary.BigEndian.Uint32(profileID[12:]))
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1, $2)`, profileLockSpace, key); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1, $2)`, profileLockSpace, key)
	}()

	return fn()
}
