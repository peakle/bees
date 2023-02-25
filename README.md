# Bees

[![Tests](https://github.com/peakle/bees/workflows/Tests/badge.svg)](https://github.com/peakle/bees/blob/master/.github/workflows/ci.yml)
[![codecov](https://codecov.io/gh/peakle/bees/branch/master/graph/badge.svg)](https://codecov.io/gh/peakle/bees)
[![Go Report Card](https://goreportcard.com/badge/github.com/peakle/bees)](https://goreportcard.com/report/github.com/peakle/bees)
[![Go Reference](https://pkg.go.dev/badge/github.com/peakle/bees.svg)](https://pkg.go.dev/github.com/peakle/bees)

Bees - simple and lightweight worker pool for go.

## Benchmarks:

### [10m tasks and 500k workers](https://github.com/peakle/bees/blob/master/pool_bench_test.go):

#### WorkerPool:

<img width="900" alt="WorkerPoolBench" src="https://user-images.githubusercontent.com/27820873/133930212-806c5918-4b30-4950-8139-326317ce3a56.png">

<b>only 37MB used for 500k workers pool</b>

#### Goroutines:

<img width="900" alt="GoroutinesBench" src="https://user-images.githubusercontent.com/27820873/133930166-d34b6dcf-b9f0-4275-93ec-08ecdb988e1f.png">

#### Semaphore:

<img width="900" alt="SemaphoreBench" src="https://user-images.githubusercontent.com/27820873/133930179-25495409-65cb-447a-ab06-72698412c646.png">

## Examples:

```go
package main

import (
	"context"
	"fmt"

	"github.com/peakle/bees"
)

// Example - demonstrate pool usage
func Example() {
	pool := bees.Create(context.Background())
	defer pool.Close()

	t := 1
	task := func(ctx context.Context) {
		t++
		fmt.Println(t)
	}
	pool.Submit(task)
	pool.Submit(task)
	pool.Submit(task)

	pool.Wait()
	// Output:
	// 1
	// 2
	// 3
}

```
