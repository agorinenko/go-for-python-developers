package lrucache_test

import (
	"fmt"
	"sync"
	"testing"

	"practice/lrucache"
)

func TestGetPutDeleteLen(t *testing.T) {
	c := lrucache.New(10)

	if n := c.Len(); n != 0 {
		t.Fatalf("Len() пустого кэша = %d, want 0", n)
	}
	if v, ok := c.Get("missing"); ok || v != "" {
		t.Fatalf("Get(missing) = (%q, %v), want промах", v, ok)
	}

	c.Put("a", "1")
	c.Put("b", "2")
	if n := c.Len(); n != 2 {
		t.Fatalf("Len() = %d, want 2", n)
	}
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = (%q, %v), want (\"1\", true)", v, ok)
	}

	c.Delete("a")
	if n := c.Len(); n != 1 {
		t.Fatalf("Len() после Delete = %d, want 1", n)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) после Delete — попадание, want промах")
	}

	c.Delete("missing") // удаление несуществующего — не ошибка
	if n := c.Len(); n != 1 {
		t.Fatalf("Len() после Delete(missing) = %d, want 1", n)
	}
}

func TestEvictionLRU(t *testing.T) {
	c := lrucache.New(2)
	c.Put("a", "1")
	c.Put("b", "2")
	c.Put("c", "3") // переполнение: вытесняется a (наименее недавно использованный)

	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a): ожидалось вытеснение ключа a")
	}
	if v, ok := c.Get("b"); !ok || v != "2" {
		t.Fatalf("Get(b) = (%q, %v), want (\"2\", true)", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != "3" {
		t.Fatalf("Get(c) = (%q, %v), want (\"3\", true)", v, ok)
	}

	c.Get("b")      // b использован недавно — он выживет
	c.Put("d", "4") // вытесняется c, а не b

	if _, ok := c.Get("c"); ok {
		t.Fatal("Get(c): ожидалось вытеснение ключа c")
	}
	if v, ok := c.Get("b"); !ok || v != "2" {
		t.Fatalf("Get(b) = (%q, %v), want (\"2\", true)", v, ok)
	}
	if v, ok := c.Get("d"); !ok || v != "4" {
		t.Fatalf("Get(d) = (%q, %v), want (\"4\", true)", v, ok)
	}
}

func TestPutUpdatesRecency(t *testing.T) {
	c := lrucache.New(2)
	c.Put("a", "1")
	c.Put("b", "2")
	c.Put("a", "10") // существующий ключ: значение обновилось, recency — тоже
	c.Put("c", "3")  // вытесняется b: a недавно обновлялся

	if v, ok := c.Get("a"); !ok || v != "10" {
		t.Fatalf("Get(a) = (%q, %v), want (\"10\", true)", v, ok)
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("Get(b): ожидалось вытеснение ключа b")
	}
	if v, ok := c.Get("c"); !ok || v != "3" {
		t.Fatalf("Get(c) = (%q, %v), want (\"3\", true)", v, ok)
	}
	if n := c.Len(); n != 2 {
		t.Fatalf("Len() = %d, want 2", n)
	}
}

func TestConcurrentAccess(t *testing.T) {
	const (
		cap        = 8
		keys       = 16
		goroutines = 100
	)
	c := lrucache.New(cap)

	// каждый ключ получает ровно одно каноническое значение — так чтение
	// можно проверять без гонок на «кто последний записал»
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", g%keys)
			c.Put(key, "v-"+key)
			for i := 0; i < keys; i++ {
				k := fmt.Sprintf("k-%d", i)
				if v, ok := c.Get(k); ok && v != "v-"+k {
					t.Errorf("значение искажено: %s = %q, want %q", k, v, "v-"+k)
				}
			}
		}(g)
	}
	wg.Wait()

	// инварианты после конкурентной нагрузки
	if n := c.Len(); n != cap {
		t.Fatalf("Len() = %d, want %d (кэш должен быть заполнен ровно по вместимости)", n, cap)
	}
	for i := 0; i < keys; i++ {
		k := fmt.Sprintf("k-%d", i)
		if v, ok := c.Get(k); ok && v != "v-"+k {
			t.Fatalf("значение искажено: %s = %q, want %q", k, v, "v-"+k)
		}
	}
}

func TestZeroCap(t *testing.T) {
	c := lrucache.New(0)
	c.Put("a", "1")
	if n := c.Len(); n != 0 {
		t.Fatalf("Len() = %d, want 0", n)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) в кэше нулевой вместимости — попадание, want промах")
	}
}
