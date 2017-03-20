package main

import "testing"

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
