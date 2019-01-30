package timestampMap

import (
	"sync"
	"whapp-irc/whapp"
)

// A Map contains the last timestamp per chat ID.
type Map struct {
	mutex sync.RWMutex
	m     map[string]int64
}

// New contains a new Map.
func New() *Map {
	return &Map{
		m: make(map[string]int64),
	}
}

// Get returns the value attachted to the given chat ID.
func (tm *Map) Get(id whapp.ID) (val int64, found bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	val, found = tm.m[id.String()]
	return val, found
}

// Set sets the timestamp for the given chat ID.
func (tm *Map) Set(id whapp.ID, val int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m[id.String()] = val
}

// GetCopy returns a copy of the internal map.
func (tm *Map) GetCopy() map[string]int64 {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	res := make(map[string]int64)
	for k, v := range tm.m {
		res[k] = v
	}
	return res
}

// Swap swaps the internal map for the given one.
func (tm *Map) Swap(m map[string]int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m = m
}

// Length returns the amount of timestamps stored in the map.
func (tm *Map) Length() int {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	return len(tm.m)
}
