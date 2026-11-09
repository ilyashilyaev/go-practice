package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newTestServer — сервер с пустым хранилищем и логгером в никуда.
func newTestServer() *Server {
	return NewServer(NewMemStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// do отправляет запрос через полный Handler (с middleware) и возвращает ответ.
func do(t *testing.T, srv *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		listing   Listing
		wantField string // "" — ошибки быть не должно
	}{
		{name: "ok", listing: Listing{Title: "Flat", City: "Moscow", Price: 100}},
		{name: "empty title", listing: Listing{City: "Moscow", Price: 100}, wantField: "title"},
		{name: "empty city", listing: Listing{Title: "Flat", Price: 100}, wantField: "city"},
		{name: "negative price", listing: Listing{Title: "Flat", City: "Moscow", Price: -1}, wantField: "price"},
		{name: "zero price ok", listing: Listing{Title: "Flat", City: "Moscow", Price: 0}},
		// TODO: добавь ещё кейсы валидации (минимум 8 всего по заданию 6).
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.listing)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			var ve *ValidationError
			if !asValidation(err, &ve) {
				t.Fatalf("validate() = %v, want *ValidationError", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}

func TestCreateAndGet(t *testing.T) {
	srv := newTestServer()

	rec := do(t, srv, http.MethodPost, "/listings", `{"title":"Flat","city":"Moscow","price":100}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201 (body: %s)", rec.Code, rec.Body)
	}
	var created Listing
	mustJSON(t, rec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatal("created listing has empty ID")
	}

	rec = do(t, srv, http.MethodGet, "/listings/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	var got Listing
	mustJSON(t, rec.Body.Bytes(), &got)
	if got != created {
		t.Errorf("GET = %+v, want %+v", got, created)
	}
}

func TestGetNotFound(t *testing.T) {
	srv := newTestServer()
	rec := do(t, srv, http.MethodGet, "/listings/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	srv := newTestServer()
	rec := do(t, srv, http.MethodPost, "/listings", `{"city":"Moscow","price":100}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestListFilterAndPaging(t *testing.T) {
	srv := newTestServer()
	seed := []string{
		`{"title":"A","city":"Moscow","price":100}`,
		`{"title":"B","city":"Moscow","price":300}`,
		`{"title":"C","city":"Kazan","price":200}`,
	}
	for _, b := range seed {
		if rec := do(t, srv, http.MethodPost, "/listings", b); rec.Code != http.StatusCreated {
			t.Fatalf("seed failed: %d %s", rec.Code, rec.Body)
		}
	}

	rec := do(t, srv, http.MethodGet, "/listings?city=Moscow&max_price=150", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got []Listing
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Title != "A" {
		t.Errorf("filter result = %+v, want only A", got)
	}

	rec = do(t, srv, http.MethodGet, "/listings?limit=2", "")
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("limit=2 returned %d items", len(got))
	}
}

// TestStoreConcurrent гоняет хранилище из многих горутин: go test -race.
func TestStoreConcurrent(t *testing.T) {
	store := NewMemStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := store.Create(Listing{Title: "T", City: "Moscow", Price: 100})
			if err != nil {
				t.Errorf("Create: %v", err)
				return
			}
			_, _ = store.Get(l.ID)
			_ = store.List(Filter{})
		}()
	}
	wg.Wait()
}

// --- helpers ---

func mustJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
}

// asValidation — короткая обёртка над errors.As.
func asValidation(err error, target **ValidationError) bool {
	return errors.As(err, target)
}
