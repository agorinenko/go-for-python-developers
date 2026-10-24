package api

import (
	"net/http"
)

// Коды ошибок единой схемы (контракт CAPSTONE.md, поле error.code).
const (
	codeBadRequest = "bad_request"
	codeNotFound   = "not_found"
	codeConflict   = "conflict"
	codeInternal   = "internal"
)

// WriteError пишет ошибку в единой схеме {"error": {"code", "message"}}
// и выставляет HTTP-статус по маппингу доменных ошибок (лекция, раздел
// «Единая схема ошибок»):
//
//	domain.ErrOrderNotFound     → 404 not_found
//	domain.ErrInvalidItem       → 400 bad_request
//	domain.ErrEmptyCustomer     → 400 bad_request
//	domain.ErrNoItems           → 400 bad_request
//	domain.ErrInvalidTransition → 409 conflict
//	всё остальное               → 500 internal, message = "internal error"
//
// Правила:
//  1. для 4xx message — текст ошибки (детали можно и нужно показывать);
//  2. для 5xx — ТОЛЬКО generic-сообщение: детали (адреса, пароли, SQL)
//     не должны утекать клиенту;
//  3. Content-Type: application/json;
//  4. статус выставляй ДО записи тела (w.WriteHeader первым).
//
// TODO: реализуй через errors.Is по цепочке проверок и
// json.NewEncoder(w).Encode(ErrorResponse{...}) для тела.
// Сейчас заглушка ничего не пишет — тесты падают на статусе и пустом теле.
func WriteError(w http.ResponseWriter, err error) {
	// TODO: реализуй.
	_ = err
}
