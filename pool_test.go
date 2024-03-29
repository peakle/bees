package bees

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var emptyTask = func(ctx context.Context) {}

func TestClose(t *testing.T) {
	t.Parallel()

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("uncatched panic: %+v", err)
		}
	}()

	pool := Create(context.Background(), WithCapacity(3), WithKeepAlive(50*time.Second), WithJitter(1))
	pool.SetLogger(log.Default())

	for i := 0; i < 100; i++ {
		go pool.Submit(emptyTask)
		go pool.Submit(emptyTask)
	}

	pool.Close()
}

func TestShutdownWithStacked(t *testing.T) {
	t.Parallel()

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("uncatched panic: %+v", err)
		}
	}()

	task := func(ctx context.Context) {
		select {
		case <-time.After(time.Hour):
		case <-ctx.Done():
		}
	}
	pool := Create(context.Background(), WithCapacity(1), WithKeepAlive(time.Hour))

	var wg sync.WaitGroup
	// fill up worker pool by tasks
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.Submit(task)
		}()
	}

	pool.Close()
	wg.Wait()
}

func TestConfigDefaultValues(t *testing.T) {
	t.Parallel()

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("uncatched panic: %+v", err)
		}
	}()

	pool := Create(context.Background(), WithKeepAlive(0), WithJitter(0), WithCapacity(0))

	if pool.cfg.KeepAliveTimeout == 0 || pool.cfg.TimeoutJitter == 0 || pool.cfg.Capacity == 0 {
		t.Fail()
	}
}

func TestRecoverAfterPanicOnSingleWorker(t *testing.T) {
	t.Parallel()

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("uncatched panic: %+v", err)
		}
	}()

	checkCh := make(chan struct{})
	task := func(ctx context.Context) {
		checkCh <- struct{}{}
		panic("aaaaaa")
	}
	pool := Create(context.Background(), WithKeepAlive(time.Hour), WithJitter(10), WithCapacity(1))
	pool.SetLogger(log.New(io.Discard, "", 0))

	pool.Submit(task)
	<-checkCh // check first execution
	pool.Submit(task)
	<-checkCh // check second try to execute
}

func TestWorkerExpiration(t *testing.T) {
	t.Parallel()

	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("uncatched panic: %+v", err)
		}
	}()

	pool := Create(context.Background(), WithKeepAlive(10*time.Millisecond), WithJitter(1), WithCapacity(1))

	for i := 0; i < 100; i++ {
		pool.Submit(emptyTask)
	}
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(pool.activeWorkers) != 0 || atomic.LoadInt64(pool.freeWorkers) != 0 {
		t.Fatalf("active workers found")
	}
}

func TestWait(t *testing.T) {
	t.Parallel()
	const testCount = 1000

	counter := ptrOfInt64(0)
	pool := Create(
		context.Background(),
		WithJitter(1),
		WithCapacity(testCount),
	)

	task := func(ctx context.Context) { time.Sleep(time.Millisecond); atomic.AddInt64(counter, 1) }
	stopper := ptrOfInt64(0)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for atomic.LoadInt64(stopper) == 0 {
			time.Sleep(time.Microsecond)
		}
		for i := 0; i < testCount; i++ {
			pool.Submit(task)
		}
		wg.Done()
	}()
	atomic.AddInt64(stopper, 1)

	wg.Wait()
	pool.Wait()

	if actualCounter := atomic.LoadInt64(counter); actualCounter != testCount {
		t.Fatalf("counter not equal: expected: %d, actual: %d", testCount, actualCounter)
	}
}

func TestOnPanic(t *testing.T) {
	t.Parallel()
	const testCount = 1000

	pool := Create(
		context.Background(),
		WithJitter(1),
		WithCapacity(testCount),
		WithTaskLen(0),
	)
	pool.SetLogger(log.New(io.Discard, "", 0))

	task := func(ctx context.Context) { time.Sleep(time.Millisecond); panic("foo") }
	stopper := ptrOfInt64(0)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for atomic.LoadInt64(stopper) == 0 {
			time.Sleep(time.Microsecond)
		}
		for i := 0; i < testCount; i++ {
			pool.Submit(task)
		}
		wg.Done()
	}()
	atomic.AddInt64(stopper, 1)

	wg.Wait()
	pool.Wait()

	taskCount := atomic.LoadInt64(pool.taskCount)
	taskChLen := len(pool.taskCh)
	if taskCount != 0 || taskChLen != 0 {
		t.Fatalf("unconsistent task count: taskCount: %d, channel len: %d", taskCount, taskChLen)
	}

	pool.Close()

	if aw := atomic.LoadInt64(pool.activeWorkers); aw != 0 {
		t.Fatalf("unconsistent active workers count: expected zero, actual: %d", pool.activeWorkers)
	}

	if fw := atomic.LoadInt64(pool.freeWorkers); fw != 0 {
		t.Fatalf("unconsistent free workers count: expected zero, actual: %d", fw)
	}

	if atomic.LoadInt64(pool.isClosed) != 1 {
		t.Fatalf("isClosed must be one")
	}
}

func TestCloseGracefullyByTimeout(t *testing.T) {
	t.Parallel()

	counter := ptrOfInt64(0)
	pool := Create(
		context.Background(),
		WithJitter(1),
		WithKeepAlive(time.Minute),
		WithCapacity(1),
		WithGracefulTimeout(3*time.Second),
		WithTaskLen(2),
	)

	task := func(ctx context.Context) {
		time.Sleep(time.Second)
		atomic.AddInt64(counter, 1)
	}
	for i := 0; i < 100; i++ {
		go pool.Submit(task)
	}
	start := time.Now()
	pool.CloseGracefully()

	if end := time.Since(start); end.Round(time.Second) > 6*time.Second {
		t.Fatalf("too big wait time: %s", end)
	}
}

func TestScale(t *testing.T) {
	t.Parallel()

	pool := Create(context.Background(), WithCapacity(1))

	var wg sync.WaitGroup
	wg.Add(1)

	task := func(ctx context.Context) {
		time.Sleep(time.Second)
	}

	go func() {
		defer wg.Done()
		pool.Submit(task)
	}()

	wg.Wait()
	pool.Wait()

	if wCap := atomic.LoadInt64(pool.workersCapacity); wCap != 1 {
		t.Fatalf("wrong capacity: %d", wCap)
	}

	pool.Scale(100)
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			go pool.Submit(task)
		}
	}()

	if wCap := atomic.LoadInt64(pool.workersCapacity); wCap != 101 {
		t.Fatalf("wrong capacity: %d", wCap)
	}

	wg.Wait()
	pool.Wait()

	pool.Scale(-100)
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < 2; i++ {
			go pool.Submit(task)
		}
	}()

	if wCap := atomic.LoadInt64(pool.workersCapacity); wCap != 1 {
		t.Fatalf("wrong capacity: %d", wCap)
	}

	aw := atomic.LoadInt64(pool.activeWorkers)
	fw := atomic.LoadInt64(pool.freeWorkers)
	if fw < 0 && aw < 0 {
		t.Fatalf("must be grater than zero, because some workers still alive after decrease worker count")
	}

	wg.Wait()
	pool.Wait()
}
