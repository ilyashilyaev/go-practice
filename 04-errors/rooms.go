// Package rooms — in-memory хранилище номеров с бронированием.
//
// Главное в задании не хранилище, а ошибки: какие бывают, как их
// оборачивать и как вызывающий код отличает одну от другой.
package rooms

import (
	"errors"
	"fmt"
)

// Sentinel-ошибки: заранее созданные значения, с которыми сравнивают через errors.Is.
// По соглашению имя начинается с Err, текст — с маленькой буквы и без точки.
var (
	ErrRoomNotFound = errors.New("room not found")
	ErrRoomTaken    = errors.New("room already taken")
)

// ValidationError — входные данные некорректны. Field — имя поля, Reason — что не так.
//
// Это не sentinel, а ТИП ошибки: у каждого экземпляра свои данные (какое поле).
// Достают его не через errors.Is, а через errors.As.
type ValidationError struct {
	Field  string
	Reason string
}

// Error реализует интерфейс error: "invalid <Field>: <Reason>".
func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// Store хранит номера: ключ — ID номера, значение — ID пользователя,
// который его забронировал ("" — свободен).
//
// Конкурентный доступ здесь не нужен — mutex появится в задании 5.
type Store struct {
	rooms map[string]string
}

// NewStore создаёт хранилище со свободными номерами roomIDs.
func NewStore(roomIDs ...string) *Store {
	s := &Store{rooms: make(map[string]string, len(roomIDs))}
	for _, id := range roomIDs {
		s.rooms[id] = ""
	}
	return s
}

// Book бронирует номер roomID для пользователя userID.
//
// Возможные ошибки (все — с контекстом, какой номер):
//   - *ValidationError, если roomID или userID пустые;
//   - ErrRoomNotFound, если такого номера нет;
//   - ErrRoomTaken, если номер уже забронирован (кем угодно, даже тем же пользователем).
//
// Проверка: go test -run TestStore_Book -v ./04-errors
func (s *Store) Book(roomID string, userID string) error {
	if roomID == "" {
		return &ValidationError{Field: "roomID", Reason: "must not be empty"}
	}
	if userID == "" {
		return &ValidationError{Field: "userID", Reason: "must not be empty"}
	}
	owner, ok := s.rooms[roomID]
	if !ok {
		return fmt.Errorf("book room %q: %w", roomID, ErrRoomNotFound)
	}
	if owner != "" {
		return fmt.Errorf("book room %q: %w", roomID, ErrRoomTaken)
	}
	s.rooms[roomID] = userID
	return nil
}
