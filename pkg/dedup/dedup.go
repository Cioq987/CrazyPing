package dedup

import (
	"sync"
	"time"
)

// Deduplicator keeps track of recently seen sequence IDs and discards duplicates.
type Deduplicator struct {
	mu      sync.RWMutex
	seen    map[uint32]int64 // seq -> seenAt (unix nano)
	ttlNano int64
}

// New creates a Deduplicator with the given retention duration.
func New(ttl time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen:    make(map[uint32]int64, 8192),
		ttlNano: ttl.Nanoseconds(),
	}
	go d.janitor(ttl)
	return d
}

// IsDuplicate returns true if seq was already seen within the TTL window.
// If it's new, it registers the sequence number and returns false.
func (d *Deduplicator) IsDuplicate(seq uint32) bool {
	now := time.Now().UnixNano()

	d.mu.Lock()
	defer d.mu.Unlock()

	if seenAt, exists := d.seen[seq]; exists {
		if now-seenAt < d.ttlNano {
			return true
		}
	}

	d.seen[seq] = now
	return false
}

// janitor periodically removes expired sequences to keep memory bounded.
func (d *Deduplicator) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now().UnixNano()
		d.mu.Lock()
		for seq, seenAt := range d.seen {
			if now-seenAt > d.ttlNano {
				delete(d.seen, seq)
			}
		}
		d.mu.Unlock()
	}
}
