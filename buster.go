// Package buster provides a generic framework for load testing.
//
// Specifically, Buster allows you to run a job at a specific concurrency level
// and a fixed rate while monitoring throughput and latency.
//
// The generic nature of Buster makes it suitable for load testing many
// different systems—HTTP servers, databases, RPC services, etc.
package buster

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codahale/hdrhistogram"
)

// A Generator is a type passed to Job instances to manage load generation.
type Generator struct {
	hist                     *hdrhistogram.Histogram
	success, failure         *uint64
	warmup, duration, period time.Duration
}

// Do generates load using the given function.
func (gen *Generator) Do(f func() error) error {
	ticker := time.NewTicker(gen.period)
	defer ticker.Stop()

	timeout := time.After(gen.duration + gen.warmup)
	warmed := time.Now().Add(gen.warmup)

	for {
		select {
		case start := <-ticker.C:
			if err := f(); err != nil {
				if start.After(warmed) {
					atomic.AddUint64(gen.failure, 1)
				}
				continue
			}
			if start.After(warmed) {
				elapsed := us(time.Now().Sub(start))
				if err := gen.hist.RecordCorrectedValue(elapsed, us(gen.period)); err != nil {
					log.Println(err)
				}
				atomic.AddUint64(gen.success, 1)
			}
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

func (r Result) String() string {
	out := bytes.NewBuffer(nil)

	fmt.Fprintf(out,
		"%d successes, %d failures, %d errors, %f ops/sec\n",
		r.Success, r.Failure, len(r.Errors),
		float64(r.Success)/r.Elapsed.Seconds(),
	)

	for _, b := range r.Latency.CumulativeDistribution() {
		fmt.Fprintf(out, "p%f = %fms\n", b.Quantile, float64(b.ValueAt)/10000)
	}

	return out.String()
}

// A Job is an arbitrary task.
type Job func(id int, generator *Generator) error

// A Bench is place where jobs are done.
type Bench struct {
	Warmup, Duration, MinLatency, MaxLatency time.Duration
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
				warmup:   b.Warmup,
			}

			started.Wait()
			errors <- job(id, gen)
			timings <- gen.hist
		}(i)
	}

	started.Done()
	finished.Wait()
	result.Elapsed = b.Duration

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
