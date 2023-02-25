package bees

import (
	"time"
)

// config - configuration struct for WorkerPool used only in Create method
type config struct {
	Capacity         int64
	KeepAliveTimeout time.Duration
	TimeoutJitter    int64 // additional random timeout in ms
	GracefulTimeout  time.Duration
}

type logger interface {
	Printf(format string, args ...interface{})
}

type Option interface {
	apply(cfg *config)
}

type jitterOption int

func (j jitterOption) apply(cfg *config) {
	cfg.TimeoutJitter = int64(j)
}

// WithJitter - add timeout jitter for workers
func WithJitter(jitter int) Option {
	return jitterOption(jitter)
}

var WithoutJitter = jitterOption(1)

type keepAliveOption time.Duration

func (k keepAliveOption) apply(cfg *config) {
	cfg.KeepAliveTimeout = time.Duration(k)
}

// WithKeepAlive - add keep alive timeout for workers
func WithKeepAlive(k time.Duration) Option {
	return keepAliveOption(k)
}

type capacity int

func (m capacity) apply(cfg *config) {
	cfg.Capacity = int64(m)
}

// WithCapacity - add max capacity for worker pool
func WithCapacity(m int64) Option {
	return capacity(m)
}

type gracefulTimeout time.Duration

func WithGracefulTimeout(t time.Duration) Option {
	return gracefulTimeout(t)
}

func (t gracefulTimeout) apply(cfg *config) {
	cfg.GracefulTimeout = time.Duration(t)
}
