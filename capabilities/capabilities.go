package capabilities

import (
	"context"
	"strings"
	"sync"
)

// A Map contains the capabilities as negotiated with a connection.
type Map struct {
	finishedCh chan interface{}

	mu sync.RWMutex

	started  bool
	finished bool

	caps []string
}

// MakeMap returns a new Map.
func MakeMap() *Map {
	return &Map{
		finishedCh: make(chan interface{}),
	}
}

// Add adds the given capability to the map.
func (cm *Map) Add(cap string) {
	cap = strings.TrimSpace(cap)

	cm.mu.Lock()
	cm.caps = append(cm.caps, cap)
	cm.mu.Unlock()
}

// Has returns whether or not the given capability has been negotiated.
func (cm *Map) Has(cap string) bool {
	cap = strings.ToUpper(cap)

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, x := range cm.caps {
		if strings.ToUpper(x) == cap {
			return true
		}
	}
	return false
}

// List returns all the negotiated capabilities.
func (cm *Map) List() []string {
	res := make([]string, len(cm.caps))

	cm.mu.RLock()
	copy(res, cm.caps)
	cm.mu.RUnlock()

	return res
}

// StartNegotiation notifies others that the negotiation process has started.
func (cm *Map) StartNegotiation() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.started || cm.finished {
		return false
	}

	cm.started = true
	return true
}

// FinishNegotiation notifies others that the negotiation process has finished.
func (cm *Map) FinishNegotiation() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.finished {
		return false
	}

	cm.finished = true
	close(cm.finishedCh)
	return true
}

// StartedNegotiation returns whether or not the negotiation process has been
// started (yet).
func (cm *Map) StartedNegotiation() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.started
}

// WaitNegotiation waits for the negotiation process to finish, or for the
// context to cancel, whatever happens first.
func (cm *Map) WaitNegotiation(ctx context.Context) (started, ok bool) {
	if cm.StartedNegotiation() {
		select {
		case <-ctx.Done():
			return true, false
		case <-cm.finishedCh:
			return true, true
		}
	}

	return false, true
}
