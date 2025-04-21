package pkg

import "sync"

type RWMap struct {
	mux   sync.RWMutex
	rwMap map[string]string
}

func (m *RWMap) Get(key string) (string, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	val, ok := m.rwMap[key]

	return val, ok
}

func (m *RWMap) Set(key, val string) bool {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.rwMap[key] = val

	return true
}

func (m *RWMap) GetKeys() []string {
	keys := []string{}
	m.mux.RLock()
	defer m.mux.RUnlock()

	for key := range m.rwMap {
		keys = append(keys, key)
	}

	return keys
}

func NewRWMap() *RWMap {
	rwMap := RWMap{
		rwMap: make(map[string]string),
	}

	return &rwMap
}
