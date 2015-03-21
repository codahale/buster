// Package buster provides a generic framework for load testing.
//
// Specifically, Buster allows you to run a set of independent jobs either at a
// specific concurrency level or at a number of concurrency levels (e.g., from 1
// to 100 goroutines, increasing by 10 each) while monitoring throughput and
// latency.
//
// The generic nature of Buster makes it suitable for load testing many
// different systems—HTTP servers, databases, RPC services, etc. It also has
// built-in support for modeling using the Universal Scalability Law, which
// allows you to determine the maximum throughput of a system without testing it
// to overload.
package buster

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codahale/hdrhistogram"
)

// A Generator is a type passed to Job instances to manage load generation.
type Generator struct {
	hist             *hdrhistogram.Histogram
	success, failure *uint64
	duration, period time.Duration
}

// Do generates load using the given function.
func (gen *Generator) Do(f func() error) error {
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

// A Result is returned after a number of concurrent jobs are run.
type Result struct {
	Concurrency      int
	Elapsed          time.Duration
	Success, Failure uint64
	Latency          *hdrhistogram.Histogram
	Errors           []error
}

// A Job is an arbitrary task.
type Job func(id int, generator *Generator) error

// A Bench is place where jobs are done.
type Bench struct {
	Duration, MinLatency, MaxLatency time.Duration
}

// Run runs the given job at the given concurrency level, at the given rate,
// returning a set of results with aggregated latency and throughput
// measurements.
func (b Bench) Run(concurrency, rate int, job Job) Result {
	var started, finished sync.WaitGroup
	started.Add(1)
	finished.Add(concurrency)

	result := Result{
		Concurrency: concurrency,
		Latency:     hdrhistogram.New(us(b.MinLatency), us(b.MaxLatency), 5),
	}
	timings := make(chan *hdrhistogram.Histogram, concurrency)
	errors := make(chan error, concurrency)

	workerRate := float64(concurrency) / float64(rate)
	period := time.Duration((workerRate)*1000000) * time.Microsecond

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer finished.Done()

			gen := &Generator{
				hist:     hdrhistogram.New(us(b.MinLatency), us(b.MaxLatency), 5),
				success:  &result.Success,
				failure:  &result.Failure,
				period:   period,
				duration: b.Duration,
			}

			started.Wait()
			errors <- job(id, gen)
			timings <- gen.hist
		}(i)
	}

	start := time.Now()
	started.Done()
	finished.Wait()
	result.Elapsed = time.Now().Sub(start)

	close(timings)
	for v := range timings {
		result.Latency.Merge(v)
	}

	close(errors)
	for e := range errors {
		if e != nil {
			result.Errors = append(result.Errors, e)
		}
	}

	return result
}

func us(d time.Duration) int64 {
	return d.Nanoseconds() / 1000
}
