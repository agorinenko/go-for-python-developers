// Package lrucache — потокобезопасный LRU-кэш на sync.RWMutex.
// В капстоуне на его основе строится orders/internal/cache (неделя 4).
package lrucache

import (
	"sync"
)

// Cache — LRU-кэш значений по строковым ключам.
//
// Внутреннее состояние (data и lru) защищено RWMutex:
//   - чтение (Len) — RLock;
//   - всё, что меняет порядок lru (Get, Put, Delete), — полный Lock.
type Cache struct {
	cap  int
	mu   sync.RWMutex
	data map[string]string // ключ -> значение
	lru  []string          // ключи в порядке использования: lru[0] — самый давний
}

// New создаёт кэш вместимостью cap. cap < 1 — кэш ничего не хранит:
// Put — no-op, Get всегда промах, Len == 0.
func New(cap int) *Cache {
	return &Cache{
		cap:  cap,
		data: make(map[string]string),
	}
}

// Get возвращает значение по ключу и признак попадания.
// Попадание обновляет recency ключа (переносит его в конец lru).
//
// TODO: реализуй ПОД LOCK (Get меняет порядок lru — RLock недостаточно):
//  1. поиск в data; промах — ("", false);
//  2. попадание — перенеси ключ в конец lru и верни (value, true).
func (c *Cache) Get(key string) (string, bool) {
	// TODO: реализуй. Заглушка: всегда промах — тесты падают на попаданиях.
	return "", false
}

// Put сохраняет значение. Если ключ уже есть — обновляет значение и recency.
// При переполнении вытесняется наименее недавно использованный ключ (lru[0]).
//
// TODO: реализуй ПОД LOCK:
//  1. cap < 1 — no-op (хранить нечего);
//  2. ключ есть — обнови значение и перенеси ключ в конец lru;
//  3. ключа нет — добавь в data и в конец lru; если len(lru) > cap —
//     удали lru[0] из lru и из data (вытеснение LRU).
func (c *Cache) Put(key, value string) {
	// TODO: реализуй. Заглушка.
}

// Delete удаляет ключ, если он есть. Несуществующий ключ — не ошибка.
//
// TODO: реализуй ПОД LOCK: удали ключ из data и из lru.
func (c *Cache) Delete(key string) {
	// TODO: реализуй. Заглушка.
}

// Len — количество ключей в кэше.
//
// TODO: реализуй ПОД RLock — это чистое чтение, писателей ждать не нужно.
func (c *Cache) Len() int {
	// TODO: реализуй. Заглушка.
	return 0
}
