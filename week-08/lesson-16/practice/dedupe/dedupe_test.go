package dedupe_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"practice/dedupe"
)

func TestSeenOnce(t *testing.T) {
	s := dedupe.NewSeen()

	if !s.Once("o-1") {
		t.Fatal("Once: первый вызов ключа должен вернуть true")
	}
	if s.Once("o-1") {
		t.Fatal("Once: повторный вызов ключа должен вернуть false")
	}
	if !s.Once("o-2") {
		t.Fatal("Once: другой ключ должен вернуть true")
	}
}

func TestSeenConcurrent(t *testing.T) {
	s := dedupe.NewSeen()

	const goroutines = 100
	var trues atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if s.Once("shared-key") {
				trues.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := trues.Load(); got != 1 {
		t.Fatalf("Once вернул true %d раз из %d горутин, want 1 (ровно одна)", got, goroutines)
	}
	if s.Once("shared-key") {
		t.Fatal("после конкурентной нагрузки ключ должен оставаться запомненным")
	}
}
