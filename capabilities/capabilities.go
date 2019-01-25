package capabilities

import (
	"context"
	"strings"
	"sync"
)

type CapabilitiesMap struct {
	finishedCh chan interface{}

	m sync.RWMutex

	started  bool
	finished bool

	caps []string
}

func MakeCapabilitiesMap() *CapabilitiesMap {
	return &CapabilitiesMap{
		finishedCh: make(chan interface{}),
	}
}

func (cm *CapabilitiesMap) Add(cap string) {
	cm.m.Lock()
	defer cm.m.Unlock()

	cap = strings.TrimSpace(cap)
	cm.caps = append(cm.caps, cap)
}

func (cm *CapabilitiesMap) Has(cap string) bool {
	cm.m.RLock()
	defer cm.m.RUnlock()

	cap = strings.ToUpper(cap)
	for _, x := range cm.caps {
		if strings.ToUpper(x) == cap {
			return true
		}
	}
	return false
}

func (cm *CapabilitiesMap) List() []string {
	cm.m.RLock()
	defer cm.m.RUnlock()

	res := make([]string, len(cm.caps))
	copy(res, cm.caps)
	return res
}

func (cm *CapabilitiesMap) StartNegotiation() bool {
	cm.m.Lock()
	defer cm.m.Unlock()

	if cm.started || cm.finished {
		return false
	}

	cm.started = true
	return true
}

func (cm *CapabilitiesMap) FinishNegotiation() bool {
	cm.m.Lock()
	defer cm.m.Unlock()

	if cm.finished {
		return false
	}

	cm.finished = true
	close(cm.finishedCh)
	return true
}

func (cm *CapabilitiesMap) StartedNegotiation() bool {
	cm.m.RLock()
	defer cm.m.RUnlock()

	return cm.started
}

func (cm *CapabilitiesMap) WaitNegotiation(ctx context.Context) (started, ok bool) {
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
