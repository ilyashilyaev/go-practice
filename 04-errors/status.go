package rooms

import (
	"errors"
	"net/http"
)

// StatusCode превращает ошибку Book в HTTP-код ответа:
//
//	nil              -> 200 OK
//	*ValidationError -> 400 Bad Request
//	ErrRoomNotFound  -> 404 Not Found
//	ErrRoomTaken     -> 409 Conflict
//	остальное        -> 500 Internal Server Error
//
// errors.As/Is разворачивают цепочку %w, поэтому код работает и для ошибок,
// обёрнутых вызывающим кодом. Это основа обработки ошибок в хендлерах задания 5.
func StatusCode(err error) int {
	var ve *ValidationError
	switch {
	case err == nil:
		return http.StatusOK
	case errors.As(err, &ve):
		return http.StatusBadRequest
	case errors.Is(err, ErrRoomNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrRoomTaken):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
