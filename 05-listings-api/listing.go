// Package main — HTTP API объявлений о жилье с хранением в памяти.
//
// Задание 5: net/http без фреймворков, middleware, graceful shutdown.
// Хранилище спрятано за интерфейсом Store, чтобы в задании 2-го этапа
// заменить его на PostgreSQL, не трогая хендлеры.
package main

import (
	"errors"
)

// ErrNotFound — объявления с таким id нет. Sentinel, проверяется errors.Is.
var ErrNotFound = errors.New("listing not found")

// ValidationError — входные данные некорректны (поле Field, причина Reason).
// Как в задании 4: тип ошибки, достаётся через errors.As, маппится в 422.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return "invalid " + e.Field + ": " + e.Reason
}

// Listing — объявление о жилье.
type Listing struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	City  string `json:"city"`
	Price int    `json:"price"` // цена за ночь, в рублях
}

// Filter — параметры выборки для List (из query-строки).
// Нулевое значение поля означает «не фильтровать».
type Filter struct {
	City     string
	MaxPrice int
	Limit    int
	Offset   int
}

// Patch — частичное обновление. Указатель nil означает «поле не меняем»;
// ненулевой указатель — новое значение (в том числе на пустое/ноль).
// Так PATCH отличает «не передано» от «передано пустым» — обычный JSON
// в структуру значений этого не умеет.
type Patch struct {
	Title *string `json:"title"`
	City  *string `json:"city"`
	Price *int    `json:"price"`
}

// Store — хранилище объявлений. Хендлеры зависят только от него.
//
// Контракт ошибок:
//   - Get/Update/Delete несуществующего id -> ErrNotFound;
//   - Create/Update с плохими данными -> *ValidationError;
//   - List ошибок не возвращает (пустой срез — это не ошибка).
type Store interface {
	Create(l Listing) (Listing, error)
	Get(id string) (Listing, error)
	List(f Filter) []Listing
	Update(id string, p Patch) (Listing, error)
	Delete(id string) error
}
