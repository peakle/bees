package bees

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	_ = 1 << (10 * iota)
	_
	MiB
)

const (
	poolSize      = 500000
	sleep         = 10
	benchRunTimes = 10000000
)

// memory usage: 1034 MB
func BenchmarkSemaphore(b *testing.B) {
	runtime.GOMAXPROCS(2)

	var wg sync.WaitGroup
	sema := make(chan struct{}, poolSize)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(benchRunTimes)
		for j := 0; j < benchRunTimes; j++ {
			sema <- struct{}{}
			go func() {
				demoFunc()
				<-sema
				wg.Done()
			}()
		}
	}
	wg.Wait()
	b.StopTimer()

	b.Logf("memory usage:%d MB", checkMem())
}

// memory usage: 963 MB
func BenchmarkGoroutines(b *testing.B) {
	runtime.GOMAXPROCS(2)

	var wg sync.WaitGroup
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(benchRunTimes)
		for j := 0; j < benchRunTimes; j++ {
			go func() {
				demoFunc()
				wg.Done()
			}()
		}
	}
	wg.Wait()
	b.StopTimer()

	b.Logf("memory usage:%d MB", checkMem())
}

// memory usage: 27 MB
func BenchmarkWorkerPool(b *testing.B) {
	runtime.GOMAXPROCS(2)

	var wg sync.WaitGroup

	p := Create(context.Background(), WithCapacity(poolSize), WithKeepAlive(5*time.Second))
	defer func() {
		p.Close()
	}()
	task := func(ctx context.Context) {
		defer wg.Done()
		demoFunc()
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(benchRunTimes)
		for j := 0; j < benchRunTimes; j++ {
			p.Submit(task)
		}
	}
	wg.Wait()
	b.StopTimer()

	b.Logf("memory usage:%d MB", checkMem())
}

func checkMem() uint64 {
	var curMem uint64
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	return curMem
}

func demoFunc() {
	time.Sleep(time.Duration(sleep) * time.Millisecond)
}
