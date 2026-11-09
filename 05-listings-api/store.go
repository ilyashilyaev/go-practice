package main

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
)

// MemStore — потокобезопасное хранилище в памяти.
// RWMutex: Get/List берут RLock и идут параллельно, запись — эксклюзивный Lock.
type MemStore struct {
	mu     sync.RWMutex
	items  map[string]Listing
	nextID int
}

// NewMemStore создаёт пустое хранилище.
func NewMemStore() *MemStore {
	return &MemStore{items: make(map[string]Listing)}
}

// validate проверяет поля объявления. Общая для Create и Update.
func validate(l Listing) error {
	switch {
	case l.Title == "":
		return &ValidationError{Field: "title", Reason: "must not be empty"}
	case l.City == "":
		return &ValidationError{Field: "city", Reason: "must not be empty"}
	case l.Price < 0:
		return &ValidationError{Field: "price", Reason: "must be >= 0"}
	}
	return nil
}

// Create валидирует, присваивает id и сохраняет объявление.
func (s *MemStore) Create(l Listing) (Listing, error) {
	if err := validate(l); err != nil {
		return Listing{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	l.ID = strconv.Itoa(s.nextID)
	s.items[l.ID] = l
	return l, nil
}

// Get возвращает объявление по id или ErrNotFound.
func (s *MemStore) Get(id string) (Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.items[id]
	if !ok {
		return Listing{}, fmt.Errorf("get %q: %w", id, ErrNotFound)
	}
	return l, nil
}

// List возвращает объявления под фильтр, отсортированные по id (стабильный порядок).
func (s *MemStore) List(f Filter) []Listing {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Listing, 0)
	for _, l := range s.items {
		if f.City != "" && l.City != f.City {
			continue
		}
		if f.MaxPrice > 0 && l.Price > f.MaxPrice {
			continue
		}
		out = append(out, l)
	}

	slices.SortFunc(out, func(a, b Listing) int {
		ai, _ := strconv.Atoi(a.ID)
		bi, _ := strconv.Atoi(b.ID)
		return ai - bi
	})

	if f.Offset >= len(out) {
		return []Listing{}
	}
	out = out[f.Offset:]
	if f.Limit > 0 && f.Limit < len(out) {
		out = out[:f.Limit]
	}
	return out
}

// Update применяет ненулевые поля Patch, валидирует и сохраняет.
func (s *MemStore) Update(id string, p Patch) (Listing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.items[id]
	if !ok {
		return Listing{}, fmt.Errorf("update %q: %w", id, ErrNotFound)
	}
	if p.Title != nil {
		l.Title = *p.Title
	}
	if p.City != nil {
		l.City = *p.City
	}
	if p.Price != nil {
		l.Price = *p.Price
	}
	if err := validate(l); err != nil {
		return Listing{}, err
	}
	s.items[id] = l
	return l, nil
}

// Delete удаляет объявление по id или возвращает ErrNotFound.
func (s *MemStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("delete %q: %w", id, ErrNotFound)
	}
	delete(s.items, id)
	return nil
}

// atoiDefault — число из query-параметра; при пустой или плохой строке возвращает def.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
