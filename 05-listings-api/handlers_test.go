package main

// Тесты API через httptest.NewServer (реальный HTTP).
// NewRecorder-тесты — в server_test.go.

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newLiveServer поднимает реальный HTTP-сервер на localhost и возвращает его
// базовый URL. t.Cleanup сам закроет его в конце теста.
func newLiveServer(t *testing.T) string {
	t.Helper()
	srv := NewServer(NewMemStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts.URL
}

// postListing — помощник: создаёт объявление через реальный HTTP, возвращает его.
func postListing(t *testing.T, base, body string) Listing {
	t.Helper()
	resp, err := http.Post(base+"/listings", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", resp.StatusCode)
	}
	var l Listing
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return l
}

// TestLive_PATCH: меняем одно поле, остальные не трогаются.
func TestLive_PATCH(t *testing.T) {
	base := newLiveServer(t)
	created := postListing(t, base, `{"title":"Flat","city":"Moscow","price":100}`)

	req, err := http.NewRequest(http.MethodPatch, base+"/listings/"+created.ID,
		bytes.NewBufferString(`{"price":200}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200", resp.StatusCode)
	}
	var got Listing
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Price != 200 || got.Title != "Flat" || got.City != "Moscow" {
		t.Errorf("after PATCH = %+v, want price=200, title/city прежние", got)
	}
}

func TestLive_DELETE(t *testing.T) {
	t.Skip("TODO: DELETE + повторный GET 404")
}

// TestValidationCases: POST с разными телами -> ожидаемый код.
func TestValidationCases(t *testing.T) {
	base := newLiveServer(t)

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{name: "valid", body: `{"title":"A","city":"Moscow","price":100}`, wantCode: http.StatusCreated},
		{name: "empty title", body: `{"city":"Moscow","price":100}`, wantCode: http.StatusUnprocessableEntity},
		{name: "empty city", body: `{"title":"A","price":100}`, wantCode: http.StatusUnprocessableEntity},
		{name: "negative price", body: `{"title":"A","city":"Moscow","price":-1}`, wantCode: http.StatusUnprocessableEntity},
		{name: "broken json", body: `{broken`, wantCode: http.StatusUnprocessableEntity},
		{name: "empty body", body: `{}`, wantCode: http.StatusUnprocessableEntity},
		{name: "zero price ok", body: `{"title":"B","city":"Kazan","price":0}`, wantCode: http.StatusCreated},
		{name: "extra fields ignored", body: `{"title":"B","city":"Kazan","price":10,"foo":"x"}`, wantCode: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(base+"/listings", "application/json", bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tt.wantCode {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}
		})
	}
}

// TestLive_NotFound: GET/PATCH/DELETE несуществующего id -> 404.
func TestLive_NotFound(t *testing.T) {
	base := newLiveServer(t)

	tests := []struct {
		method, body string
	}{
		{http.MethodGet, ""},
		{http.MethodPatch, `{"price":200}`},
		{http.MethodDelete, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, base+"/listings/nope",
				bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s: %v", tt.method, err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s status = %d, want 404", tt.method, resp.StatusCode)
			}
		})
	}
}
