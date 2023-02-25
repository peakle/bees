package bees

import (
	"context"
	"fmt"
	"time"
)

// Example - demonstrate pool usage
func Example() {
	pool := Create(context.Background(),
		WithCapacity(1), WithKeepAlive(time.Minute), WithoutJitter,
	)
	defer pool.Close()

	t := 0
	task := func(context.Context) {
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
