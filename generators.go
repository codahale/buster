package buster

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/codahale/hdrhistogram"
)

// A Generator is a type passed to Job instances to manage load generation.
type Generator interface {
	// Do generates load using the given function.
	Do(f func() error) error
}

// A GeneratorType is a factory for generators.
type GeneratorType func(r reporter) Generator

// MaxThroughput returns a GeneratorType which performs the given work in a
// tight loop with no throttling.
//
// This is useful for determining saturation points, but the latency
// measurements produced by this generator will be artificially low due to
// coordinated omission (i.e., latency spikes will slow down the workers and
// produce fewer slow measurements).
func MaxThroughput(duration time.Duration) GeneratorType {
	return func(r reporter) Generator {
		return &maxThroughputGenerator{
			reporter: r,
			duration: duration,
		}
	}
}

// ConstantRate returns a GeneratorType which performs the given work at the
// given rate in Hz (operations per second).
//
// This is useful for measuring the latency of a system, as each worker will
// count the number of missed "ticks" using the HdrHistogram, compensating for
// coordinated omission effects.
func ConstantRate(duration time.Duration, Hz float64) GeneratorType {
	period := time.Duration((1/Hz)*1000000) * time.Microsecond
	return func(r reporter) Generator {
		return &constantRateGenerator{
			reporter: r,
			duration: duration,
			period:   period,
		}
	}
}

type reporter struct {
	hist             *hdrhistogram.Histogram
	success, failure *uint64
}

type maxThroughputGenerator struct {
	reporter
	duration time.Duration
}

func (gen *maxThroughputGenerator) Do(f func() error) error {
	end := time.Now().Add(gen.duration)
	for {
		start := time.Now()
		if end.Before(start) {
			return nil
		}

		if err := f(); err != nil {
			atomic.AddUint64(gen.failure, 1)
			continue
		}
		elapsed := us(time.Now().Sub(start))
		if err := gen.hist.RecordValue(elapsed); err != nil {
			log.Println(err)
		}
		atomic.AddUint64(gen.success, 1)
	}
}

type constantRateGenerator struct {
	reporter
	duration, period time.Duration
}

func (gen *constantRateGenerator) Do(f func() error) error {
	ticker := time.NewTicker(gen.period)
	defer ticker.Stop()

	timeout := time.After(gen.duration)

	for {
		select {
		case start := <-ticker.C:
			if err := f(); err != nil {
				atomic.AddUint64(gen.failure, 1)
				continue
			}
			elapsed := us(time.Now().Sub(start))
			if err := gen.hist.RecordCorrectedValue(elapsed, us(gen.duration)); err != nil {
				log.Println(err)
			}
			atomic.AddUint64(gen.success, 1)
		case <-timeout:
			return nil
		}
	}
}
