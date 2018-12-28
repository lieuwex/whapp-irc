package capabilities

import (
	"strings"
	"sync"
)

type CapabilitiesMap struct {
	m sync.RWMutex

	startedNegotiation         bool
	negotiationFinished        bool
	negotiationFinishedChannel chan bool

	caps []string
}

func MakeCapabilitiesMap() *CapabilitiesMap {
	return &CapabilitiesMap{
		negotiationFinishedChannel: make(chan bool),
	}
}

func (cm *CapabilitiesMap) AddCapability(cap string) {
	cm.m.Lock()
	defer cm.m.Unlock()

	cap = strings.TrimSpace(cap)
	cm.caps = append(cm.caps, cap)
}

func (cm *CapabilitiesMap) HasCapability(cap string) bool {
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

func (cm *CapabilitiesMap) Caps() []string {
	cm.m.RLock()
	defer cm.m.RUnlock()

	res := make([]string, len(cm.caps))
	copy(res, cm.caps)
	return res
}

func (cm *CapabilitiesMap) StartNegotiation() {
	cm.m.Lock()
	defer cm.m.Unlock()

	cm.startedNegotiation = true
}

func (cm *CapabilitiesMap) FinishNegotiation() bool {
	cm.m.Lock()
	defer cm.m.Unlock()

	if cm.negotiationFinished {
		return false
	}

	cm.negotiationFinished = true
	close(cm.negotiationFinishedChannel)
	return true
}

func (cm *CapabilitiesMap) StartedNegotiation() bool {
	cm.m.RLock()
	defer cm.m.RUnlock()

	return cm.startedNegotiation
}

func (cm *CapabilitiesMap) WaitNegotiation() {
	if cm.StartedNegotiation() {
		<-cm.negotiationFinishedChannel
	}
}
