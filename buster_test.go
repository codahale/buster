package buster_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/codahale/buster"
	"github.com/codahale/hdrhistogram"
)

func Example() {
	// 1, 10, 20 ... 90, 100
	step := buster.Log(buster.FixedStep(1, 100, 10))

	bench := buster.Bench{
		Duration:   10 * time.Second,
		MinLatency: 1 * time.Microsecond,
		MaxLatency: 1 * time.Minute,
	}

	result := bench.AutoRun(func(id int, gen *buster.Generator) error {
		client := &http.Client{}

		return gen.Do(func() error {
			resp, err := client.Get("http://www.google.com/")
			if err != nil {
				return err
			}
			io.Copy(ioutil.Discard, resp.Body)
			return resp.Body.Close()
		})
	}, buster.Log(step))

	for _, r := range result {
		for _, b := range r.Latency.CumulativeDistribution() {
			fmt.Printf(
				"p%f @ %d threads: %fms\n",
				b.Quantile,
				r.Concurrency,
				float64(b.ValueAt)/1000,
			)
		}
	}
}

func TestLog(t *testing.T) {
	step := buster.Log(buster.FixedStep(1, 100, 10))

	if v, want := step(nil), 1; v != want {
		t.Errorf("Was %d, but expected %d", v, want)
	}

	result := buster.Result{
		Latency:     hdrhistogram.New(1, 1000, 5),
		Concurrency: 1,
	}

	if v, want := step(&result), 10; v != want {
		t.Errorf("Was %d, but expected %d", v, want)
	}
}

func TestFixedStep(t *testing.T) {
	step := buster.FixedStep(1, 100, 10)

	result := buster.Result{}
	result.Concurrency = step(nil)

	levels := []int{}
	levels = append(levels, result.Concurrency)

	for result.Concurrency > 0 {
		result.Concurrency = step(&result)
		levels = append(levels, result.Concurrency)
	}

	expected := []int{1, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, -1}

	if !reflect.DeepEqual(levels, expected) {
		t.Errorf("Was %v but expected %v", levels, expected)
	}
}

func TestMaxLatency(t *testing.T) {
	step := buster.MaxLatency(99, 40*time.Microsecond, buster.FixedStep(1, 100, 5))

	result := buster.Result{
		Latency:     hdrhistogram.New(1, 1000, 5),
		Concurrency: step(nil),
	}

	levels := []int{}
	levels = append(levels, result.Concurrency)

	for result.Concurrency > 0 {
		result.Latency.RecordValue(1 << uint(result.Concurrency))
		result.Concurrency = step(&result)
		levels = append(levels, result.Concurrency)
	}

	expected := []int{1, 5, 10, -1}

	if !reflect.DeepEqual(levels, expected) {
		t.Errorf("Was %v but expected %v", levels, expected)
	}
}

func TestBenchRun(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Second,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return nil
		})
	}, 10)

	if v, want := r.Concurrency, 10; v != want {
		t.Errorf("Concurrency was %d, but expected %d", v, want)
	}
}

func TestBenchRunFailures(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Second,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return errors.New("woo hoo")
		})
	}, 10)

	if r.Failure == 0 {
		t.Errorf("Failure count was 0, but expected %d", r.Failure)
	}
}

func TestBenchRunErrors(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Second,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(func(id int, gen *buster.Generator) error {
		return errors.New("woo hoo")
	}, 10)

	if v, want := len(r.Errors), 10; v != want {
		t.Fatalf("Error count was %d, but expected %d", v, want)
	}
}

func TestBenchAutoRun(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Millisecond,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return nil
		})
	}, buster.FixedStep(1, 100, 10))

	if v, want := len(results), 11; v != want {
		t.Fatalf("Result size was %d, but expected %d", v, want)
	}
}

func TestBenchAutoRunFailures(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Millisecond,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return errors.New("woo hoo")
		})
	}, buster.FixedStep(1, 100, 10))

	if results[0].Failure == 0 {
		t.Errorf("Failure count was 0, but expected %d", results[0].Failure)
	}
}

func TestBenchAutoRunErrors(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Millisecond,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(func(id int, gen *buster.Generator) error {
		return errors.New("woo hoo")
	}, buster.FixedStep(1, 100, 10))

	if v, want := len(results[0].Errors), 1; v != want {
		t.Fatalf("Error count was %d, but expected %d", v, want)
	}
}
