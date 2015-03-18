package buster_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/codahale/buster"
	"github.com/codahale/hdrhistogram"
)

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
