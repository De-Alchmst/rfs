package rfs

import (
	"time"
)


// Rfs caches responses.
// Each request has a certain amount of TTL, which is replenished when it's used.
// TTL is periodically reduced and entry is removed when it reaches 0.
var (
	// how long between TTL reduction
	CacheFlushTimeout = 5 * time.Second
	// How much TTL does an entry have
	DefaultTTL int64 = 5 * 60 / 5
)


func cacheFlushing() {
	for {
		time.Sleep(1 * time.Second)
		flushStep(entries)
		flushStep(pidEntries)
	}
}


func flushAll() {
	for path, _ := range entries {
		delete(entries, path)
	}
}


func flushName(name string) {
	delete(entries, name)
}


func flushStep[K comparable](ent map[K]*pathEntry) {
	for key, e := range ent {
		if e.Status == entryStatusProcessing {
			continue
		}

		if e.TTL <= 0 {
			delete(ent, key)
		} else {
			e.TTL -= 1
		}
	}
}
