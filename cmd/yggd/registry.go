package main

import (
	"sync"
)

var errWorkerRegistered = registryError("cannot add worker; a worker is already registered")

type registryError string

func (e registryError) Error() string { return string(e) }

type registry struct {
	mu sync.RWMutex
	mp map[string]*worker
}

func (r *registry) init() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.mp == nil {
		r.mp = make(map[string]*worker)
	}
}

func (r *registry) set(handler string, w *worker) error {
	r.init()

	if r.get(handler) != nil {
		return errWorkerRegistered
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.mp[handler] = w

	return nil
}

func (r *registry) get(handler string) *worker {
	r.init()

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.mp[handler]
}

func (r *registry) del(handler string) {
	r.init()

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.mp, handler)
}

func (r *registry) all() map[string]*worker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make(map[string]*worker)
	for k, w := range r.mp {
		all[k] = w
	}

	return all
}
