package state

import (
	"fmt"
	"sync"
)

type ScratchStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewScratchStore() *ScratchStore {
	return &ScratchStore{
		data: make(map[string]string),
	}
}

func (s *ScratchStore) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *ScratchStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in scratch", key)
	}
	return val, nil
}

func (s *ScratchStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *ScratchStore) List() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}
