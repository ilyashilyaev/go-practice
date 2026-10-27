package notify

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// fakeNotifier — написанный вручную тестовый двойник Notifier.
// Запоминает вызовы и возвращает заранее заданную ошибку.
// Никаких библиотек для моков: в Go fake на 10 строк обычно проще и понятнее.
type fakeNotifier struct {
	err   error  // что вернуть из Notify
	calls []call // что пришло в Notify
}

type call struct {
	userID, msg string
}

func (f *fakeNotifier) Notify(_ context.Context, userID string, msg string) error {
	f.calls = append(f.calls, call{userID: userID, msg: msg})
	return f.err
}

func TestBookingService_Book(t *testing.T) {
	fake := &fakeNotifier{}
	svc := NewBookingService(fake)

	if err := svc.Book(context.Background(), "u1", "101"); err != nil {
		t.Fatalf("Book() error = %v", err)
	}

	want := []call{{userID: "u1", msg: "room 101 booked"}}
	if len(fake.calls) != 1 || fake.calls[0] != want[0] {
		t.Errorf("Notify calls = %+v, want %+v", fake.calls, want)
	}
}

func TestBookingService_BookNotifyError(t *testing.T) {
	errDown := errors.New("smtp down")
	svc := NewBookingService(&fakeNotifier{err: errDown})

	err := svc.Book(context.Background(), "u1", "101")
	if !errors.Is(err, errDown) {
		t.Fatalf("Book() error = %v, want wrapping %v", err, errDown)
	}
	if !strings.Contains(err.Error(), "u1") {
		t.Errorf("error %q should mention user id", err)
	}
}

func TestEmailNotifier(t *testing.T) {
	var out bytes.Buffer
	n := &EmailNotifier{From: "bot@example.com", Out: &out}

	if err := n.Notify(context.Background(), "u1", "hi"); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if got, want := out.String(), "email from bot@example.com to u1: hi\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestSMSNotifier(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantErr error
		wantOut string
	}{
		{name: "short", msg: "hi", wantOut: "sms to u1: hi\n"},
		{name: "exactly 160 cyrillic chars", msg: strings.Repeat("я", MaxSMSLength), wantOut: "sms to u1: " + strings.Repeat("я", MaxSMSLength) + "\n"},
		{name: "161 chars", msg: strings.Repeat("a", MaxSMSLength+1), wantErr: ErrMessageTooLong},
		// TODO: добавь кейс — пустое сообщение. Отправлять или считать ошибкой?
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := (&SMSNotifier{Out: &out}).Notify(context.Background(), "u1", tt.msg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Notify() error = %v, want %v", err, tt.wantErr)
			}
			if got := out.String(); got != tt.wantOut {
				t.Errorf("output = %q, want %q", got, tt.wantOut)
			}
		})
	}
}

func TestLogNotifier(t *testing.T) {
	var out bytes.Buffer
	n := &LogNotifier{Logger: slog.New(slog.NewTextHandler(&out, nil))}

	if err := n.Notify(context.Background(), "u1", "hi"); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	for _, want := range []string{"msg=notification", "user_id=u1", "msg=hi"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("log %q does not contain %q", out.String(), want)
		}
	}
}

func TestNotifiers_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // контекст отменён ещё до вызова

	var out bytes.Buffer
	notifiers := map[string]Notifier{
		"email": &EmailNotifier{From: "bot@example.com", Out: &out},
		"sms":   &SMSNotifier{Out: &out},
	}
	for name, n := range notifiers {
		t.Run(name, func(t *testing.T) {
			if err := n.Notify(ctx, "u1", "hi"); !errors.Is(err, context.Canceled) {
				t.Errorf("Notify() error = %v, want %v", err, context.Canceled)
			}
		})
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be sent after cancel, got %q", out.String())
	}
}

func TestMultiNotifier(t *testing.T) {
	errSMS := errors.New("sms gateway down")
	errEmail := errors.New("smtp down")

	ok := &fakeNotifier{}
	sms := &fakeNotifier{err: errSMS}
	email := &fakeNotifier{err: errEmail}
	last := &fakeNotifier{}

	m := &MultiNotifier{Notifiers: []Notifier{ok, sms, email, last}}
	err := m.Notify(context.Background(), "u1", "hi")

	// Обе ошибки доступны через errors.Is — в этом смысл errors.Join.
	if !errors.Is(err, errSMS) || !errors.Is(err, errEmail) {
		t.Errorf("Notify() error = %v, want both %v and %v", err, errSMS, errEmail)
	}
	// Ошибки в середине не остановили отправку остальным.
	for i, f := range []*fakeNotifier{ok, sms, email, last} {
		if len(f.calls) != 1 {
			t.Errorf("notifier %d called %d times, want 1", i, len(f.calls))
		}
	}
}

func TestMultiNotifier_AllOK(t *testing.T) {
	m := &MultiNotifier{Notifiers: []Notifier{&fakeNotifier{}, &fakeNotifier{}}}
	if err := m.Notify(context.Background(), "u1", "hi"); err != nil {
		t.Errorf("Notify() error = %v, want nil", err)
	}
}
