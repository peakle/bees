package bees

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool - keep information about active/free workers and task processing
type WorkerPool struct {
	activeWorkers   *int64
	freeWorkers     *int64
	taskCount       *int64
	workersCapacity *int64

	taskCh chan func(ctx context.Context)

	cfg         *config
	shutdownCtx context.Context
	cancelFunc  context.CancelFunc
	wg          *sync.WaitGroup
	isClosed    *int64

	logger logger
}

// Create - create worker pool instance
func Create(ctx context.Context, opts ...Option) *WorkerPool {
	var cfg = config{
		Capacity:         1,
		TimeoutJitter:    1000,
		KeepAliveTimeout: time.Minute,
		GracefulTimeout:  time.Minute,
	}

	for _, opt := range opts {
		opt.apply(&cfg)
	}

	if cfg.TimeoutJitter <= 0 {
		cfg.TimeoutJitter = 1
	}

	if cfg.KeepAliveTimeout <= 0 {
		cfg.KeepAliveTimeout = time.Second
	}

	if cfg.Capacity <= 0 {
		cfg.Capacity = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	return &WorkerPool{
		activeWorkers:   ptrOfInt64(0),
		freeWorkers:     ptrOfInt64(0),
		taskCount:       ptrOfInt64(0),
		workersCapacity: ptrOfInt64(cfg.Capacity),
		taskCh:          make(chan func(context.Context), 2*cfg.Capacity),
		cfg:             &cfg,
		shutdownCtx:     ctx,
		cancelFunc:      cancel,
		wg:              wg,
		logger:          log.Default(),
		isClosed:        ptrOfInt64(0),
	}
}

// SetLogger - sets logger for pool
func (wp *WorkerPool) SetLogger(logger logger) {
	wp.logger = logger
}

// Submit - submit task to pool
func (wp *WorkerPool) Submit(task func(context.Context)) {
	if atomic.LoadInt64(wp.isClosed) == 1 {
		return
	}

	wp.retrieveWorker()
	wp.taskCh <- task // TODO: may optimize blocking send
	atomic.AddInt64(wp.taskCount, 1)
}

// SubmitAsync - submit task to pool, for async better use this method
func (wp *WorkerPool) SubmitAsync(task func(context.Context)) {
	if atomic.LoadInt64(wp.isClosed) == 1 {
		return
	}

	wp.retrieveWorker()
	select {
	case wp.taskCh <- task:
		atomic.AddInt64(wp.taskCount, 1)
	case <-wp.shutdownCtx.Done():
	}
}

func (wp *WorkerPool) Wait() {
	const maxBackoff = 16
	backoff := 1

	for atomic.LoadInt64(wp.taskCount) != 0 {
		for i := 0; i < backoff; i++ {
			time.Sleep(time.Duration(backoff) * time.Microsecond)
		}
		if backoff < maxBackoff {
			backoff <<= 1
		}
	}
}

// Close - close worker pool and release all resources, not processed tasks will be thrown away
func (wp *WorkerPool) Close() {
	atomic.StoreInt64(wp.isClosed, 1)
	wp.cancelFunc()
	wp.wg.Add(-1)
	wp.wg.Wait()
}

// CloseGracefully - close worker pool and release all resources, wait until all task will be processed
func (wp *WorkerPool) CloseGracefully() {
	atomic.StoreInt64(wp.isClosed, 1)
	closed := make(chan struct{})
	go func() {
		wp.Wait()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(wp.cfg.GracefulTimeout):
	}

	wp.cancelFunc()

	wp.wg.Add(-1)
	wp.wg.Wait()
}

func (wp *WorkerPool) Scale(delta int64) {
	atomic.AddInt64(wp.workersCapacity, delta)
}

func (wp *WorkerPool) retrieveWorker() {
	max := atomic.LoadInt64(wp.workersCapacity)
	active := atomic.LoadInt64(wp.activeWorkers)
	free := atomic.LoadInt64(wp.freeWorkers)

	if free <= 1 && active < max {
		if atomic.CompareAndSwapInt64(wp.activeWorkers, active, active+1) {
			wp.spawnWorker()
		}
	}
}

func (wp *WorkerPool) spawnWorker() {
	atomic.AddInt64(wp.freeWorkers, 1)
	wp.wg.Add(1)

	go func() {
		// jitter depends on global rand state, but it's not the ok here
		jitter := func() time.Duration { return time.Millisecond * time.Duration(rand.Int63n(wp.cfg.TimeoutJitter)) }
		// https://en.wikipedia.org/wiki/Exponential_backoff
		// nolint:gosec
		ticker := time.NewTicker(wp.cfg.KeepAliveTimeout + jitter())
		defer ticker.Stop()

		defer func() {
			atomic.AddInt64(wp.freeWorkers, -1)
			atomic.AddInt64(wp.activeWorkers, -1)
			wp.wg.Done()
			if err := recover(); err != nil {
				atomic.AddInt64(wp.freeWorkers, 1)
				atomic.AddInt64(wp.taskCount, -1)
				if atomic.LoadInt64(wp.activeWorkers) == 0 {
					go wp.retrieveWorker()
				}

				wp.logger.Printf("on WorkerPool: on Process: %+v", err)
				return
			}
		}()

		for {
			select {
			case task := <-wp.taskCh:
				atomic.AddInt64(wp.freeWorkers, -1)
				task(wp.shutdownCtx)
				atomic.AddInt64(wp.freeWorkers, 1)
				atomic.AddInt64(wp.taskCount, -1)
			case <-wp.shutdownCtx.Done():
				return
			case <-ticker.C:
				return
			}
			ticker.Reset(wp.cfg.KeepAliveTimeout + jitter())
		}
	}()
}

func ptrOfInt64(i int64) *int64 {
	return &i
}
