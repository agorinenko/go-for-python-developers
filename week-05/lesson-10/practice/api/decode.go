package api

import (
	"io"

	"practice/domain"
)

// DecodeCreateOrder декодирует и валидирует тело POST /v1/orders.
// Требования (лекция, разделы «Потоковый декодинг» и «Валидация»):
//  1. json.Decoder с DisallowUnknownFields: неизвестное поле — ошибка;
//  2. ровно одно JSON-значение в потоке: после успешного Decode вызови
//     Decode(&struct{}{}) ещё раз — вторая попытка обязана вернуть io.EOF,
//     иначе в теле мусор после JSON;
//  3. синтаксическая ошибка JSON — обычная ошибка с пояснением (plain error);
//  4. валидация, ошибки оборачивай доменными sentinel через %w —
//     так WriteError отдаст правильный статус:
//     - customer_id пуст → fmt.Errorf("%w: customer_id is required",
//     domain.ErrEmptyCustomer);
//     - items пуст (нет ключа или null) → domain.ErrNoItems;
//     - у позиции quantity <= 0 или price <= 0 →
//     fmt.Errorf("%w: items[%d]: quantity must be > 0", domain.ErrInvalidItem, i)
//     (и симметрично для price).
//
// TODO: реализуй. Сейчас возвращается пустой запрос без ошибки —
// тесты на валидацию падают именно поэтому.
func DecodeCreateOrder(r io.Reader) (CreateOrderRequest, error) {
	// TODO: реализуй.
	return CreateOrderRequest{}, nil
}

// ToDomain мапит валидный запрос в доменный заказ. ID и CreatedAt
// проставит сервисный слой (domain.NewOrder), поэтому здесь заказ
// собирается «вручную»: Status = domain.StatusNew,
// Total = сумма Quantity*Price по позициям.
//
// TODO (лекция, раздел «DTO против доменных типов»):
//  1. смапь Items → []domain.OrderItem (поля совпадают один в один);
//  2. если данные невалидны — верни соответствующую доменную ошибку
//     (ErrEmptyCustomer / ErrNoItems / ErrInvalidItem), а не паникуй:
//     DecodeCreateOrder это уже проверил, но метод должен оставаться
//     корректным при вызове в обход декодера;
//  3. посчитай Total.
func (req CreateOrderRequest) ToDomain() (domain.Order, error) {
	// TODO: реализуй.
	return domain.Order{}, nil
}
