package worker_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"practice/worker"
)

func TestWithJobTimeout(t *testing.T) {
	t.Run("без дедлайна", func(t *testing.T) {
		ctx := worker.WithJobTimeout(context.Background(), time.Minute)
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("дедлайн не установлен")
		}
		now := time.Now()
		if d.Before(now.Add(50*time.Second)) || d.After(now.Add(time.Minute+time.Second)) {
			t.Fatalf("дедлайн = %v, want ~now+1m", d)
		}
	})

	t.Run("у родителя более ранний дедлайн", func(t *testing.T) {
		parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
		defer cancel()
		parentDeadline, _ := parent.Deadline()

		ctx := worker.WithJobTimeout(parent, 10*time.Millisecond)
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("дедлайн потерялся")
		}
		if !d.Equal(parentDeadline) {
			t.Fatalf("дедлайн = %v, want родительский %v (таймаут не должен продлевать жизнь контекста)", d, parentDeadline)
		}
	})

	t.Run("дедлайн позже таймаута", func(t *testing.T) {
		parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
		defer cancel()

		ctx := worker.WithJobTimeout(parent, 10*time.Millisecond)
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("дедлайн потерялся")
		}
		now := time.Now()
		if d.Before(now.Add(time.Millisecond)) || d.After(now.Add(100*time.Millisecond)) {
			t.Fatalf("дедлайн = %v, want ~now+10ms", d)
		}
	})
}

func TestRunAllJobs(t *testing.T) {
	jobs := make(chan worker.Job, 3)
	jobs <- worker.Job{ID: "j-1", Delay: 5 * time.Millisecond}
	jobs <- worker.Job{ID: "j-2", Delay: 5 * time.Millisecond}
	jobs <- worker.Job{ID: "j-3", Delay: 5 * time.Millisecond}
	close(jobs)

	results := worker.Run(context.Background(), jobs)
	if len(results) != 3 {
		t.Fatalf("Run вернул %d результатов, want 3", len(results))
	}
	for i, r := range results {
		if want := fmt.Sprintf("j-%d", i+1); r.JobID != want {
			t.Errorf("результат %d: JobID = %q, want %q", i, r.JobID, want)
		}
		if r.Err != nil {
			t.Errorf("задание %s: err = %v, want nil", r.JobID, r.Err)
		}
	}
}

func TestRunCancelPartial(t *testing.T) {
	const total = 8
	jobs := make(chan worker.Job, total)
	for i := 0; i < total; i++ {
		jobs <- worker.Job{ID: fmt.Sprintf("j-%d", i), Delay: 30 * time.Millisecond}
	}
	close(jobs)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan []worker.Result, 1)
	go func() { done <- worker.Run(ctx, jobs) }()

	select {
	case results := <-done:
		// часть заданий успела обработаться, остальные — нет
		if len(results) < 1 {
			t.Fatalf("Run вернул 0 результатов, want хотя бы один")
		}
		if len(results) >= total {
			t.Fatalf("Run обработал все %d заданий после отмены контекста", total)
		}
		for i, r := range results {
			if want := fmt.Sprintf("j-%d", i); r.JobID != want {
				t.Fatalf("результат %d: JobID = %q, want %q (порядок обработки нарушен)", i, r.JobID, want)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run не остановился после отмены контекста")
	}
}

func TestRunJobTimeout(t *testing.T) {
	jobs := make(chan worker.Job, 3)
	jobs <- worker.Job{ID: "fast-1", Delay: 5 * time.Millisecond}
	jobs <- worker.Job{ID: "slow", Delay: 2 * time.Second} // превышает JobTimeout
	jobs <- worker.Job{ID: "fast-2", Delay: 5 * time.Millisecond}
	close(jobs)

	start := time.Now()
	results := worker.Run(context.Background(), jobs)
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("Run вернул %d результатов, want 3", len(results))
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Fatalf("быстрые задания не должны иметь ошибок: %+v", results)
	}
	if !errors.Is(results[1].Err, context.DeadlineExceeded) {
		t.Fatalf("медленное задание: err = %v, want errors.Is %v", results[1].Err, context.DeadlineExceeded)
	}
	// таймаут задания не имеет права «подвешивать» весь прогон:
	// 2s работа против 100ms таймаута должна быть пропущена, а не дождана
	if elapsed > time.Second {
		t.Fatalf("Run выполнялся %v — таймаут задания не сработал", elapsed)
	}
}

func TestRunNoGoroutineLeak(t *testing.T) {
	jobs := make(chan worker.Job, 3)
	for i := 0; i < 3; i++ {
		jobs <- worker.Job{ID: fmt.Sprintf("slow-%d", i), Delay: 3 * time.Second}
	}
	close(jobs)

	baseline := runtime.NumGoroutine()
	done := make(chan []worker.Result, 1)
	go func() { done <- worker.Run(context.Background(), jobs) }()

	var results []worker.Result
	select {
	case results = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run не завершился: таймауты заданий не работают")
	}

	if len(results) != 3 {
		t.Fatalf("Run вернул %d результатов, want 3", len(results))
	}
	for _, r := range results {
		if !errors.Is(r.Err, context.DeadlineExceeded) {
			t.Fatalf("задание %s: err = %v, want errors.Is %v", r.JobID, r.Err, context.DeadlineExceeded)
		}
	}

	// после выхода из Run порождённых горутин остаться не должно:
	// «работа» по заданию могла бы спать весь Delay (3с), если бы её
	// запускали в отдельной горутине
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline+2 {
		if time.Now().After(deadline) {
			t.Fatalf("утечка горутин: было %d, стало %d", baseline, runtime.NumGoroutine())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
