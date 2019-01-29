package timestampMap

import "sync"

type Map struct {
	mutex sync.RWMutex
	m     map[string]int64
}

func New() *Map {
	return &Map{
		m: make(map[string]int64),
	}
}

func (tm *Map) Get(key string) (val int64, found bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	val, found = tm.m[key]
	return val, found
}

func (tm *Map) Set(key string, val int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m[key] = val
}

func (tm *Map) GetCopy() map[string]int64 {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	res := make(map[string]int64)
	for k, v := range tm.m {
		res[k] = v
	}
	return res
}

func (tm *Map) Swap(m map[string]int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m = m
}

func (tm *Map) Length() int {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	return len(tm.m)
}
