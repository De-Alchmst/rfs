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

		for path, ent := range entries {
			if ent.TTL <= 0 {
				delete(entries, path)
			} else {
				ent.TTL -= 1
			}
		}
	}
}


func flushAll() {
	for path, _ := range entries {
		delete(entries, path)
	}
}
