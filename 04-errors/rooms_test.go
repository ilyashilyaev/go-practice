package rooms

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "userID", Reason: "must not be empty"}
	if got, want := err.Error(), "invalid userID: must not be empty"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestStore_Book(t *testing.T) {
	tests := []struct {
		name      string
		roomID    string
		userID    string
		wantIs    error  // sentinel, который должен найти errors.Is (nil — не проверять)
		wantField string // поле ValidationError, которое должен найти errors.As ("" — не ждём)
		wantInMsg string // подстрока в тексте ошибки — проверка контекста
	}{
		{name: "свободный номер", roomID: "101", userID: "u1"},
		{name: "номера нет", roomID: "999", userID: "u1", wantIs: ErrRoomNotFound, wantInMsg: `"999"`},
		{name: "номер занят", roomID: "102", userID: "u1", wantIs: ErrRoomTaken, wantInMsg: `"102"`},
		{name: "тот же пользователь повторно", roomID: "102", userID: "u2", wantIs: ErrRoomTaken},
		{name: "пустой roomID", roomID: "", userID: "u1", wantField: "roomID"},
		{name: "пустой userID", roomID: "101", userID: "", wantField: "userID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Свежее хранилище на каждый кейс: 101 свободен, 102 уже у u2.
			s := NewStore("101", "102")
			if err := s.Book("102", "u2"); err != nil {
				t.Fatalf("setup: Book(102, u2) error = %v", err)
			}

			err := s.Book(tt.roomID, tt.userID)

			wantErr := tt.wantIs != nil || tt.wantField != ""
			if !wantErr {
				if err != nil {
					t.Fatalf("Book() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Book() error = nil, want error")
			}

			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(%v, %v) = false", err, tt.wantIs)
			}
			if tt.wantField != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("errors.As(%v, *ValidationError) = false", err)
				}
				if ve.Field != tt.wantField {
					t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.wantField)
				}
			}
			if !strings.Contains(err.Error(), tt.wantInMsg) {
				t.Errorf("error %q should contain %q", err, tt.wantInMsg)
			}
		})
	}
}

func TestStore_BookMarksRoomTaken(t *testing.T) {
	s := NewStore("101")
	if err := s.Book("101", "u1"); err != nil {
		t.Fatalf("first Book() error = %v", err)
	}
	// Если Book не записал бронь, второй вызов пройдёт — а не должен.
	if err := s.Book("101", "u2"); !errors.Is(err, ErrRoomTaken) {
		t.Errorf("second Book() error = %v, want %v", err, ErrRoomTaken)
	}
}

func TestStatusCode(t *testing.T) {
	s := NewStore("101", "102")
	_ = s.Book("102", "u2")

	// Ошибки берём из настоящего Book и оборачиваем ещё раз — как сделает хендлер
	// в задании 5. errors.Is/As должны найти исходную ошибку сквозь обе обёртки.
	wrap := func(err error) error { return fmt.Errorf("handler: %w", err) }

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "нет ошибки", err: nil, want: http.StatusOK},
		{name: "валидация", err: wrap(s.Book("", "u1")), want: http.StatusBadRequest},
		{name: "не найден", err: wrap(s.Book("999", "u1")), want: http.StatusNotFound},
		{name: "занят", err: wrap(s.Book("102", "u1")), want: http.StatusConflict},
		{name: "неизвестная", err: errors.New("disk on fire"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusCode(tt.err); got != tt.want {
				t.Errorf("StatusCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
