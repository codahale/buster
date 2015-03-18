package buster_test

import (
	"errors"
	"testing"
	"time"

	"github.com/codahale/buster"
)

func TestMaxThroughput(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Second),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, func(id int, gen buster.Generator) error {
		return gen.Do(func() error {
			return nil
		})
	})

	if v, want := r.Concurrency, 10; v != want {
		t.Errorf("Concurrency was %d, but expected %d", v, want)
	}
}

func TestMaxThroughputFailures(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Second),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, func(id int, gen buster.Generator) error {
		return gen.Do(func() error {
			return errors.New("woo hoo")
		})
	})

	if r.Failure == 0 {
		t.Errorf("Failure count was 0, but expected %d", r.Failure)
	}
}

func TestConstantRate(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.ConstantRate(1*time.Second, 1000),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, func(id int, gen buster.Generator) error {
		return gen.Do(func() error {
			return nil
		})
	})

	if v, want := r.Concurrency, 10; v != want {
		t.Errorf("Concurrency was %d, but expected %d", v, want)
	}
}

func TestConstantRateFailures(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.ConstantRate(1*time.Second, 1000),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, func(id int, gen buster.Generator) error {
		return gen.Do(func() error {
			return errors.New("woo hoo")
		})
	})

	if r.Failure == 0 {
		t.Errorf("Failure count was 0, but expected %d", r.Failure)
	}
}
