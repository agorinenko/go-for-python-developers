// Package dedupe — дедупликация входящих событий: идемпотентный консьюмер
// саги payments (CAPSTONE.md, неделя 8 — дедупликация по order_id).
package dedupe

import "sync"

// Seen — потокобезопасное множество «уже виденных» ключей.
type Seen struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

// NewSeen создаёт пустое множество.
func NewSeen() *Seen {
	return &Seen{keys: make(map[string]struct{})}
}

// Once возвращает true при ПЕРВОМ появлении ключа и false при повторных.
// Потокобезопасно: при конкурентном вызове из многих горутин с одним ключом
// true увидит ровно одна.
//
// TODO: реализуй (лекция, раздел «Идемпотентные консьюмеры»):
// под mu проверить, есть ли ключ в keys; если нет — записать и вернуть true,
// иначе false.
func (s *Seen) Once(key string) bool {
	// TODO: реализуй. Заглушка: все ключи считаются повторными.
	return false
}
