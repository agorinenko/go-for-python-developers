package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"practice/pipeline"
)

// stageMark — этап-трансформация 1:1: проставляет статус, порядок сохраняет.
func stageMark(status string) pipeline.Stage {
	return func(ctx context.Context, in <-chan pipeline.Order, out chan<- pipeline.Order) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case o, ok := <-in:
				if !ok {
					return nil
				}
				o.Status = status
				out <- o
			}
		}
	}
}

// stageAddTax — этап-трансформация: добавляет percent% к Total.
func stageAddTax(percent int64) pipeline.Stage {
	return func(ctx context.Context, in <-chan pipeline.Order, out chan<- pipeline.Order) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case o, ok := <-in:
				if !ok {
					return nil
				}
				o.Total += o.Total * percent / 100
				out <- o
			}
		}
	}
}

// errPipelineBroken — ошибка этапа для теста аварийной остановки.
var errPipelineBroken = errors.New("pipeline: stage broken")

// stageFailOn возвращает этап, падающий на заказе с заданным ID.
func stageFailOn(id string) pipeline.Stage {
	return func(ctx context.Context, in <-chan pipeline.Order, out chan<- pipeline.Order) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case o, ok := <-in:
				if !ok {
					return nil
				}
				if o.ID == id {
					return errPipelineBroken
				}
				out <- o
			}
		}
	}
}

// feedOrders возвращает канал с n заказами и закрывает его сразу после записи.
func feedOrders(n int) <-chan pipeline.Order {
	ch := make(chan pipeline.Order, n)
	for i := 0; i < n; i++ {
		ch <- pipeline.Order{ID: fmt.Sprintf("o-%d", i), Status: "new", Total: 1_000}
	}
	close(ch)
	return ch
}

func TestRunPipeline(t *testing.T) {
	in := feedOrders(20)

	out, err := pipeline.Run(context.Background(), in, stageAddTax(10), stageMark("validated"))
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out) != 20 {
		t.Fatalf("Run вернул %d заказов, want 20", len(out))
	}
	for i, o := range out {
		if o.ID != fmt.Sprintf("o-%d", i) {
			t.Fatalf("порядок нарушен: на позиции %d ID = %q, want %q", i, o.ID, fmt.Sprintf("o-%d", i))
		}
		if o.Status != "validated" {
			t.Errorf("заказ %s: Status = %q, want %q (второй этап не применился?)", o.ID, o.Status, "validated")
		}
		if o.Total != 1_100 {
			t.Errorf("заказ %s: Total = %d, want %d (первый этап не применился?)", o.ID, o.Total, 1_100)
		}
	}
}

func TestRunNoStages(t *testing.T) {
	in := feedOrders(3)

	out, err := pipeline.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("Run без этапов вернул %d заказов, want 3", len(out))
	}
	for i, o := range out {
		if o.ID != fmt.Sprintf("o-%d", i) {
			t.Fatalf("порядок нарушен: на позиции %d ID = %q", i, o.ID)
		}
		if o.Status != "new" || o.Total != 1_000 {
			t.Errorf("заказ %s изменён при проходе без этапов: %+v", o.ID, o)
		}
	}
}

func TestRunEmpty(t *testing.T) {
	in := make(chan pipeline.Order)
	close(in)

	out, err := pipeline.Run(context.Background(), in, stageMark("validated"))
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("Run на пустом входе вернул %d заказов, want 0", len(out))
	}
}

func TestRunStageError(t *testing.T) {
	in := feedOrders(20)

	out, err := pipeline.Run(context.Background(), in, stageFailOn("o-7"), stageMark("validated"))
	if err == nil {
		t.Fatal("Run: ошибка этапа потеряна")
	}
	if !errors.Is(err, errPipelineBroken) {
		t.Fatalf("Run error = %v, want errors.Is %v", err, errPipelineBroken)
	}
	if len(out) > 20 {
		t.Fatalf("Run вернул %d заказов при входе 20", len(out))
	}
}

func TestRunCancel(t *testing.T) {
	// канал не закрывается и не получает заказов: только отмена контекста
	// может остановить конвейер
	in := make(chan pipeline.Order)
	started := make(chan struct{})
	var startedOnce sync.Once
	blockingStage := func(ctx context.Context, _ <-chan pipeline.Order, _ chan<- pipeline.Order) error {
		startedOnce.Do(func() { close(started) }) // воркеров несколько — закрываем один раз
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	type runResult struct {
		out []pipeline.Order
		err error
	}
	done := make(chan runResult, 1)
	go func() {
		out, err := pipeline.Run(ctx, in, blockingStage)
		done <- runResult{out: out, err: err}
	}()

	select {
	case <-started:
	case res := <-done:
		t.Fatalf("Run завершился, не запустив этап: out=%v err=%v", res.out, res.err)
	case <-time.After(2 * time.Second):
		t.Fatal("этап так и не запустился")
	}
	cancel()

	select {
	case res := <-done:
		if !errors.Is(res.err, context.Canceled) {
			t.Fatalf("Run error = %v, want errors.Is %v", res.err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run не остановился после отмены контекста")
	}
}
