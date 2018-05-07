package lockmap

import "sync"

// A LockMap is used to lock various things represented by strings.
// Items are never removed from the internal map, so LockaMap shouldn't be used
// for a large amount of keys.
type LockMap struct {
	mutex sync.RWMutex
	m     map[string]*sync.RWMutex
}

// New makes a new LockMap.
func New() *LockMap {
	return &LockMap{
		m: make(map[string]*sync.RWMutex),
	}
}

func (lm *LockMap) getMutex(str string) (mutex *sync.RWMutex, found bool) {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	mutex, found = lm.m[str]
	return mutex, found
}
func (lm *LockMap) addMutex(str string) *sync.RWMutex {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	mutex := &sync.RWMutex{}
	lm.m[str] = mutex
	return mutex
}
func (lm *LockMap) getLock(locker sync.Locker) (unlockFn func()) {
	locker.Lock()
	return func() {
		locker.Unlock()
	}
}

// Lock locks the given str for writing and returns a function to unlock the
// lock.
func (lm *LockMap) Lock(str string) (unlockFn func()) {
	mutex, found := lm.getMutex(str)
	if !found {
		mutex = lm.addMutex(str)
	}

	return lm.getLock(mutex)
}

// RLock locks the given str for reading and returns a function to unlock the
// lock.
func (lm *LockMap) RLock(str string) (unlockFn func()) {
	mutex, found := lm.getMutex(str)
	if !found {
		mutex = lm.addMutex(str)
	}

	return lm.getLock(mutex.RLocker())
}
