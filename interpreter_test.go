package main

import "testing"

func benchmarkProgram(path string, b *testing.B) {
	for i := 0; i < b.N; i++ {
		interpretTree(path)
	}
}

func BenchmarkParallel(b *testing.B) {
	benchmarkProgram("parallel.csp", b)
}

func BenchmarkPhilosophers(b *testing.B) {
	benchmarkProgram("philosophers.csp", b)
}

func BenchmarkClientServer(b *testing.B) {
	benchmarkProgram("clientserver.csp", b)
}

func testAllConsumed(path string, t *testing.T) {
	remaining := interpretTree(path)

	if remaining != nil {
		errorFmt := "All events should be executed in %s. " +
			"The events %v were not."
		t.Errorf(errorFmt, path, remaining)
	}
}

func TestParallel(t *testing.T) {
	testAllConsumed("parallel.csp", t)
}

func TestChannels(t *testing.T) {
	testAllConsumed("channels.csp", t)
}

func TestPhilosophers(t *testing.T) {
	testAllConsumed("philosophers.csp", t)
}
