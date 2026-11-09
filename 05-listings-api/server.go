package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Server связывает хранилище и HTTP-роутинг.
type Server struct {
	store  Store
	logger *slog.Logger
}

// NewServer создаёт сервер.
func NewServer(store Store, logger *slog.Logger) *Server {
	return &Server{store: store, logger: logger}
}

// Handler строит http.Handler: роутинг на стандартном ServeMux + цепочка middleware.
// recover снаружи — ловит панику и в logging тоже; цепочка разворачивается изнутри наружу.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /listings", s.handleList)
	mux.HandleFunc("POST /listings", s.handleCreate)
	mux.HandleFunc("GET /listings/{id}", s.handleGet)
	mux.HandleFunc("PATCH /listings/{id}", s.handleUpdate)
	mux.HandleFunc("DELETE /listings/{id}", s.handleDelete)

	var h http.Handler = mux
	h = s.withLogging(h)
	h = withRequestID(h)
	h = s.withRecover(h)
	return h
}

// writeJSON пишет v как JSON с заданным кодом. Заголовки и код — до первого Write.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// errorBody — единый JSON-формат ошибки: {"error": "..."}.
type errorBody struct {
	Error string `json:"error"`
}

// writeError переводит ошибку в HTTP-ответ: код по типу ошибки + тело errorBody.
// Для 500 наружу отдаётся общий текст, детали пишутся только в лог.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{Error: err.Error()})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
	default:
		s.logger.Error("internal error", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal error"})
	}
}

// decodeJSON читает тело запроса в dst; битый JSON — это ошибка клиента (422).
func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return &ValidationError{Field: "body", Reason: err.Error()}
	}
	return nil
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := Filter{
		City:     q.Get("city"),
		MaxPrice: atoiDefault(q.Get("max_price"), 0),
		Limit:    atoiDefault(q.Get("limit"), 0),
		Offset:   atoiDefault(q.Get("offset"), 0),
	}
	writeJSON(w, http.StatusOK, s.store.List(f))
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var l Listing
	if err := decodeJSON(r, &l); err != nil {
		s.writeError(w, err)
		return
	}
	created, err := s.store.Create(l)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	l, err := s.store.Get(r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var p Patch
	if err := decodeJSON(r, &p); err != nil {
		s.writeError(w, err)
		return
	}
	updated, err := s.store.Update(r.PathValue("id"), p)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Delete(r.PathValue("id")); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
