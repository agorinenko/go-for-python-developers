package domain

// ListFilter — фильтр списка заказов.
// В капстоуне этот тип объявлен в internal/service (контракт «Неделя 2»);
// в практике он лежит в domain, чтобы модуль оставался автономным.
// При переносе в капстоун — используйте service.ListFilter (поля те же).
type ListFilter struct {
	CustomerID string
	Status     OrderStatus
	Limit      int
}
