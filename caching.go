package rfs

import (
	"time"
)


var (
	// 5 minutes, flushed every 5 seconds
	CacheFlushTimeout = 5 * time.Second
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
