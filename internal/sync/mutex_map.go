package sync

import "sync"

// RWMutexMap is a concurrency-safe map, using a sync.RWMutex to lock a backing
// map when accessing values.
type RWMutexMap[T any] struct {
	mu sync.RWMutex
	mp map[string]T
}

func (m *RWMutexMap[T]) init() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mp == nil {
		m.mp = make(map[string]T)
	}
}

// Set locks the map, setting the key k to the value t.
func (m *RWMutexMap[T]) Set(k string, t T) {
	m.init()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.mp[k] = t
}

// Get sets a read lock on the map, retrieving the value for k. A second return
// value indicates whether the key was present in the map.
func (m *RWMutexMap[T]) Get(k string) (T, bool) {
	m.init()

	m.mu.RLock()
	defer m.mu.RUnlock()

	t, has := m.mp[k]

	return t, has
}

// Del sets a read-write lock on the map and deletes the value for k from it.
func (m *RWMutexMap[T]) Del(k string) {
	m.init()

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.mp, k)
}

// Visit read-locks the map and calls the function f for each member.
func (m *RWMutexMap[T]) Visit(f func(k string, v T)) {
	m.init()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, t := range m.mp {
		f(k, t)
	}

}
