package buster

import (
	"log"
	"time"
)

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
