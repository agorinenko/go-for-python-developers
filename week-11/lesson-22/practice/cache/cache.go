// Package cache — слой cache-aside поверх медленного источника (Backend)
// с TTL и защитой от thundering herd (singleflight).
// Контракт зафиксирован: сигнатуры менять нельзя.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Backend — источник данных (в капстоуне — репозиторий заказов на PostgreSQL).
// Set и Del — операции над кэш-хранилищем (в капстоуне — Redis): через них
// кэш прогревается и инвалидируется.
type Backend interface {
	// Get возвращает значение и true, если ключ есть в источнике.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Set сохраняет значение с временем жизни ttl.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Del удаляет ключ (инвалидация).
	Del(ctx context.Context, key string) error
}

// entry — запись локального кэша со сроком жизни.
type entry struct {
	value     []byte
	expiresAt time.Time
}

// Cache — cache-aside кэш: локальная карта значений (L1) + Backend как
// источник (и хранилище прогретых значений), singleflight против stampede.
type Cache struct {
	backend Backend
	ttl     time.Duration

	mu   sync.Mutex
	data map[string]entry

	sf singleflight.Group
}

// NewCache создаёт кэш с заданным временем жизни записей.
func NewCache(backend Backend, ttl time.Duration) *Cache {
	return &Cache{
		backend: backend,
		ttl:     ttl,
		data:    make(map[string]entry),
	}
}

// Get возвращает значение по ключу: локальный hit — без обращения к Backend;
// промах — чтение источника, причём параллельные Get по одному ключу
// дедуплицируются через singleflight (источник вызывается ровно один раз).
//
// TODO: реализуй cache-aside (лекция, разделы «Cache-aside» и «singleflight»):
//  1. быстрый путь: локальный hit — c.data[key] с неистёкшим expiresAt —
//     вернуть значение, Backend не трогать;
//  2. промах: оберни чтение в c.sf.Do(key, ...) и ВНУТРИ полёта повтори
//     проверку локального кэша — пока ты ждал слот в singleflight, другой
//     полёт мог уже заполнить кэш (иначе источник вызовется дважды);
//  3. после повторной проверки: источник вернул значение (ok=true) — заполни
//     кэш: сохрани локально (expiresAt = time.Now().Add(c.ttl)) и вызови
//     backend.Set(ctx, key, value, c.ttl);
//  4. ключа нет в источнике (ok=false): вернуть (nil, false, nil), кэш НЕ заполнять;
//  5. ошибка источника: вернуть её, кэш НЕ заполнять (ошибки не кэшируются).
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// TODO: реализуй по описанию выше.
	// Заглушка: кэш пуст, источник не вызывается — тесты на промах и singleflight падают.
	return nil, false, nil
}

// Set кладёт значение в кэш (прогрев) с TTL из конструктора.
//
// TODO: реализуй: сохрани локально (expiresAt = time.Now().Add(c.ttl))
// и вызови backend.Set(ctx, key, value, c.ttl); верни ошибку бэкенда.
func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	// TODO: реализуй по описанию выше.
	return nil
}

// Del инвалидирует ключ: следующее чтение снова пойдёт в источник.
//
// TODO: реализуй: удали ключ из локального кэша и вызови backend.Del(ctx, key);
// верни ошибку бэкенда.
func (c *Cache) Del(ctx context.Context, key string) error {
	// TODO: реализуй по описанию выше.
	return nil
}
