package api

import (
	"io"

	"practice/domain"
)

// EncodeOrder пишет заказ в JSON в формате OrderResponse.
//
// TODO (лекция, раздел «DTO против доменных типов»):
//  1. собери OrderResponse из *domain.Order: строковые статусы —
//     string(o.Status) или o.Status напрямую (OrderStatus — строковый тип,
//     кодируется как строка), позиции — []Item с теми же полями;
//  2. закодируй через json.NewEncoder(w).Encode(resp) и верни ошибку
//     кодирования, если она возникла.
//
// Сейчас заглушка ничего не пишет и возвращает nil — тесты падают
// на пустом JSON.
func EncodeOrder(w io.Writer, o *domain.Order) error {
	// TODO: реализуй.
	return nil
}
