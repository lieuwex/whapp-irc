package main

import "sync"

type TimestampMap struct {
	mutex sync.RWMutex
	m     map[string]int64
}

func MakeTimestampMap() *TimestampMap {
	return &TimestampMap{
		m: make(map[string]int64),
	}
}

func (tm *TimestampMap) Get(key string) (val int64, found bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	val, found = tm.m[key]
	return val, found
}

func (tm *TimestampMap) Set(key string, val int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m[key] = val
}

func (tm *TimestampMap) GetCopy() map[string]int64 {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	res := make(map[string]int64)
	for k, v := range tm.m {
		res[k] = v
	}
	return res
}

func (tm *TimestampMap) Swap(m map[string]int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.m = m
}
