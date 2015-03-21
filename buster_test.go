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
	// run a bench for 1 minute, tracking latencies from 1µs to 1 minute
	bench := buster.Bench{
		Duration:   1 * time.Minute,
		MinLatency: 1 * time.Microsecond,
		MaxLatency: 1 * time.Minute,
	}

	r := bench.Run(
		10,   // concurrent workers
		1000, // 1000 ops/sec total
		func(id int, gen *buster.Generator) error { // the job to be run
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

func TestBenchRun(t *testing.T) {
	bench := buster.Bench{
		Duration:   1 * time.Second,
		MinLatency: 1 * time.Millisecond,
		MaxLatency: 1 * time.Second,
	}

	r := bench.Run(10, 1000, func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return nil
		})
	})

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

	r := bench.Run(10, 1000, func(id int, gen *buster.Generator) error {
		return gen.Do(func() error {
			return errors.New("woo hoo")
		})
	})

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

	r := bench.Run(10, 100, func(id int, gen *buster.Generator) error {
		return errors.New("woo hoo")
	})

	if v, want := len(r.Errors), 10; v != want {
		t.Fatalf("Error count was %d, but expected %d", v, want)
	}
}
