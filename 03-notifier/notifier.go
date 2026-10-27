// Package notify отправляет пользователям уведомления по разным каналам.
//
// Каналы эмулируются: вместо настоящей почты или SMS сообщение пишется
// в io.Writer. Так их легко проверить в тестах.
package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"unicode/utf8"
)

// MaxSMSLength — сколько символов (не байт!) помещается в одно SMS.
const MaxSMSLength = 160

// ErrMessageTooLong возвращает SMSNotifier, если текст не помещается в одно SMS.
var ErrMessageTooLong = errors.New("message too long for sms")

// Notifier отправляет сообщение пользователю.
//
// Интерфейс маленький — один метод. В Go это норма: чем меньше интерфейс,
// тем проще его реализовать и подменить в тесте (ср. io.Reader, io.Writer).
type Notifier interface {
	Notify(ctx context.Context, userID string, msg string) error
}

// Проверки на этапе компиляции: если какой-то тип перестанет реализовывать
// Notifier, сборка упадёт здесь, а не в месте использования.
var (
	_ Notifier = (*EmailNotifier)(nil)
	_ Notifier = (*SMSNotifier)(nil)
	_ Notifier = (*LogNotifier)(nil)
	_ Notifier = (*MultiNotifier)(nil)
)

// EmailNotifier «отправляет» письмо: пишет строку в Out.
type EmailNotifier struct {
	From string
	Out  io.Writer
}

// Notify пишет в Out строку вида:
//
//	email from <From> to <userID>: <msg>
//
// Если контекст уже отменён, ничего не отправляет и возвращает его ошибку.
func (n *EmailNotifier) Notify(ctx context.Context, userID string, msg string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}
	_, err := fmt.Fprintf(n.Out, "email from %s to %s: %s\n", n.From, userID, msg)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// SMSNotifier «отправляет» SMS: пишет строку в Out.
type SMSNotifier struct {
	Out io.Writer
}

// Notify пишет в Out строку вида:
//
//	sms to <userID>: <msg>
//
// Сообщение длиннее MaxSMSLength символов не отправляется: возвращается
// ошибка, которую можно проверить через errors.Is(err, ErrMessageTooLong).
func (n *SMSNotifier) Notify(ctx context.Context, userID string, msg string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	length := utf8.RuneCountInString(msg) // символы, а не байты
	if length > MaxSMSLength {
		return fmt.Errorf("%w: %d chars", ErrMessageTooLong, length)
	}

	_, err := fmt.Fprintf(n.Out, "sms to %s: %s\n", userID, msg)
	if err != nil {
		return fmt.Errorf("send sms: %w", err)
	}
	return nil
}

// LogNotifier пишет уведомление в структурный лог.
type LogNotifier struct {
	Logger *slog.Logger
}

// Notify пишет в лог запись уровня Info с сообщением "notification"
// и атрибутами user_id и msg.
//
// Ошибок не возвращает: slog не сообщает вызывающему о сбоях записи лога.
func (n *LogNotifier) Notify(ctx context.Context, userID string, msg string) error {
	n.Logger.InfoContext(ctx, "notification", "user_id", userID, "msg", msg)

	return nil
}

// MultiNotifier отправляет сообщение через все вложенные нотификаторы.
//
// Сам он тоже Notifier — это композиция: BookingService не знает,
// один канал у него или пять.
type MultiNotifier struct {
	Notifiers []Notifier
}

// Notify вызывает Notify у каждого вложенного нотификатора.
//
// Ошибка одного канала не мешает остальным: все ошибки собираются
// и возвращаются одной через errors.Join (nil, если ошибок не было).
func (m *MultiNotifier) Notify(ctx context.Context, userID string, msg string) error {
	var errs []error
	for _, n := range m.Notifiers {
		if err := n.Notify(ctx, userID, msg); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
