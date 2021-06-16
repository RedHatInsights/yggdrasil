package yggdrasil

import "sync"

type signalEmitter struct {
	mu        sync.RWMutex
	listeners map[string][]chan interface{}
	closed    bool
}

// connect creates a channel of the given size that will be sent a value when
// the named signal is emitted.
func (s *signalEmitter) connect(name string, size int) <-chan interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listeners == nil {
		s.listeners = make(map[string][]chan interface{})
	}

	ch := make(chan interface{}, size)
	s.listeners[name] = append(s.listeners[name], ch)
	return ch
}

// disconnect loops over the slice of registered listeners, searching for ch. If
// a match is found, the channel is closed and removed from the listeners slice.
func (s *signalEmitter) disconnect(name string, ch <-chan interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listeners == nil {
		return
	}

	idx := -1
	for i, _ch := range s.listeners[name] {
		if ch == _ch {
			idx = i
			close(_ch)
			break
		}
	}

	if idx >= 0 {
		listeners := s.listeners[name]
		listeners[idx] = listeners[len(listeners)-1]
		listeners[len(listeners)-1] = nil
		listeners = listeners[:len(listeners)-1]
		s.listeners[name] = listeners
	}
}

// emit sends the given message to all channels registered under the given
// signal name.
func (s *signalEmitter) emit(name string, msg interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listeners == nil {
		s.listeners = make(map[string][]chan interface{})
	}

	if s.closed {
		return
	}

	for _, ch := range s.listeners[name] {
		ch <- msg
	}
}

// close loops over all signals and closes the channel for every listener.
func (s *signalEmitter) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		for _, l := range s.listeners {
			for _, ch := range l {
				close(ch)
			}
		}
	}
}
