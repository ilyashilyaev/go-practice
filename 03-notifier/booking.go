package notify

import (
	"context"
	"fmt"
)

// BookingService оформляет бронирования и уведомляет о них пользователя.
//
// Зависит только от интерфейса Notifier: какой канал на самом деле
// используется (почта, SMS, все сразу или fake в тесте) — решает тот,
// кто создаёт сервис. Это и есть внедрение зависимости без фреймворков.
type BookingService struct {
	notifier Notifier
}

// NewBookingService создаёт сервис с заданным каналом уведомлений.
func NewBookingService(n Notifier) *BookingService {
	return &BookingService{notifier: n}
}

// Book бронирует комнату roomID для userID и уведомляет пользователя
// сообщением "room <roomID> booked".
//
// Само хранилище комнат появится в задании 4, здесь бронирование всегда успешно.
//
// Ошибку уведомления возвращает обёрнутой: исходную можно проверить через errors.Is.
func (s *BookingService) Book(ctx context.Context, userID string, roomID string) error {
	msg := fmt.Sprintf("room %s booked", roomID)
	if err := s.notifier.Notify(ctx, userID, msg); err != nil {
		return fmt.Errorf("notify user %s: %w", userID, err)
	}
	return nil
}
