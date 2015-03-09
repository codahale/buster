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
	"github.com/codahale/usl"
)

// A Generator is a type passed to Job instances to manage load generation.
type Generator struct {
	hist             *hdrhistogram.Histogram
	success, failure *uint64
	done             time.Time
}

// Do generates load using the given function.
func (gen *Generator) Do(f func() error) error {
	for {
		start := time.Now()
		if gen.done.Before(start) {
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
	Duration               time.Duration
	MinLatency, MaxLatency time.Duration
}

// Run runs the given job at the given concurrency level, returning a set of
// results with aggregated latency and throughput measurements.
func (b Bench) Run(concurrency int, job Job) Result {
	var started, finished sync.WaitGroup
	started.Add(1)
	finished.Add(concurrency)

	result := Result{
		Concurrency: concurrency,
		Latency:     hdrhistogram.New(us(b.MinLatency), us(b.MaxLatency), 5),
	}
	timings := make(chan *hdrhistogram.Histogram, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer finished.Done()

			gen := &Generator{
				hist:    hdrhistogram.New(us(b.MinLatency), us(b.MaxLatency), 5),
				success: &result.Success,
				failure: &result.Failure,
				done:    time.Now().Add(b.Duration),
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

// AutoRun runs the given job, starting at the step's initial concurrency level
// and proceeding until the step returns a negative concurrency level. Returns
// the results for each run at each concurrency level.
func (b Bench) AutoRun(step Step, job Job) []Result {
	var results []Result

	concurrency := step(nil)
	for concurrency > 0 {
		result := b.Run(concurrency, job)
		results = append(results, result)

		concurrency = step(&result)

		if len(result.Errors) > 0 {
			break
		}
	}

	return results
}

const (
	// STOP is a value Step implementations can return to stop a run.
	STOP = -1
)

// A Step takes the last result of a run and returns the next level of
// concurrency to run the job at. The step is called with a nil pointer before
// the first run, and the step returns a negative value.
type Step func(last *Result) (nextConcurrency int)

// Log wraps the given step to write some information about the last run to
// STDERR.
func Log(step Step) Step {
	return func(r *Result) int {
		if r != nil {
			rate := float64(r.Success) / r.Elapsed.Seconds()
			p99 := float64(r.Latency.ValueAtQuantile(99)) / 1000
			log.Printf("%d: %f op/sec @ p99=%fms", r.Concurrency, rate, p99)
		}
		return step(r)
	}
}

// MaxLatency wraps the given step to stop the auto-run once the given latency
// quantile exceeds the given duration.
func MaxLatency(q float64, max time.Duration, step Step) Step {
	return func(r *Result) int {
		if r != nil {
			ns := r.Latency.ValueAtQuantile(q) * 1000
			if ns > max.Nanoseconds() {
				return STOP
			}
		}
		return step(r)
	}
}

// FixedStep steps from the given minimum to the given maximum by the given
// step.
func FixedStep(min, max, step int) Step {
	return func(r *Result) int {
		if r == nil {
			return min
		}
		if r.Concurrency >= max {
			return STOP
		}
		return (r.Concurrency/step)*step + step
	}
}

// Model builds a Universal Scalability Law model based on the given results.
func Model(results []Result) (usl.Model, error) {
	measurements := make([]usl.Measurement, 0, len(results))
	for _, r := range results {
		measurements = append(measurements, usl.Measurement{
			X: float64(r.Concurrency),
			Y: float64(r.Success) / r.Elapsed.Seconds(),
		})
	}
	return usl.Build(measurements)
}

func us(d time.Duration) int64 {
	return d.Nanoseconds() / 1000
}
