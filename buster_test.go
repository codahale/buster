package buster_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/codahale/buster"
)

func Example() {
	// run a bench for 10s, tracking latencies from 1µs to 1 minute
	bench := buster.Bench{
		// generate a constant rate of 100 req/sec per worker
		Generator:  buster.ConstantRate(1*time.Minute, 100),
		MinLatency: 1 * time.Microsecond,
		MaxLatency: 1 * time.Minute,
	}

	// run an automatic bench, using the previous step
	result := bench.AutoRun(
		// 1, 10, 20 ... 90, 100, and log on the way there
		buster.Log(buster.FixedStep(1, 100, 10)),

		// the job to be run
		func(id int, gen buster.Generator) error {
			client := &http.Client{}

			return gen.Do(func() error {
				// perform a GET request
				resp, err := client.Get("http://www.google.com/")
				if err != nil {
					return err
				}

				// read the body
				io.Copy(ioutil.Discard, resp.Body)
				return resp.Body.Close()
			})
		},
	)

	for _, r := range result {
		fmt.Printf("%d successful requests, %d failed requests", r.Success, r.Failure)

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

func TestBenchRunErrors(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Second),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, func(id int, gen buster.Generator) error {
		return errors.New("woo hoo")
	})

	if v, want := len(r.Errors), 10; v != want {
		t.Fatalf("Error count was %d, but expected %d", v, want)
	}
}

func TestBenchAutoRun(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Millisecond),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(
		buster.FixedStep(1, 100, 10),
		func(id int, gen buster.Generator) error {
			return gen.Do(func() error {
				return nil
			})
		},
	)

	if v, want := len(results), 11; v != want {
		t.Fatalf("Result size was %d, but expected %d", v, want)
	}
}

func TestBenchAutoRunFailures(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Millisecond),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(
		buster.FixedStep(1, 100, 10),
		func(id int, gen buster.Generator) error {
			return gen.Do(func() error {
				return errors.New("woo hoo")
			})
		},
	)

	if results[0].Failure == 0 {
		t.Errorf("Failure count was 0, but expected %d", results[0].Failure)
	}
}

func TestBenchAutoRunErrors(t *testing.T) {
	bench := buster.Bench{
		Generator:  buster.MaxThroughput(1 * time.Millisecond),
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	results := bench.AutoRun(
		buster.FixedStep(1, 100, 10),
		func(id int, gen buster.Generator) error {
			return errors.New("woo hoo")
		},
	)

	if v, want := len(results[0].Errors), 1; v != want {
		t.Fatalf("Error count was %d, but expected %d", v, want)
	}
}

func TestModel(t *testing.T) {
	results := []buster.Result{
		{
			Concurrency: 1,
			Elapsed:     1 * time.Second,
			Success:     100,
		},
		{
			Concurrency: 2,
			Elapsed:     1 * time.Second,
			Success:     200,
		},
		{
			Concurrency: 3,
			Elapsed:     1 * time.Second,
			Success:     300,
		},
		{
			Concurrency: 4,
			Elapsed:     1 * time.Second,
			Success:     400,
		},
		{
			Concurrency: 5,
			Elapsed:     1 * time.Second,
			Success:     400,
		},
		{
			Concurrency: 6,
			Elapsed:     1 * time.Second,
			Success:     300,
		},
	}

	model, err := buster.Model(results)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(model)
}
