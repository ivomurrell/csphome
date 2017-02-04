package main

import (
	"bufio"
	"flag"
	"log"
	"math/rand"
	"os"
	"strings"
	"text/scanner"
	"time"
)

//go:generate go tool yacc -p "csp" -o parser.go csp.y

const (
	forkConsumed = iota
	forkRunning
	forkDone
)

var traceCount int = 0

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	path := flag.String("f", "", "File path to CSP definitions.")
	flag.Parse()
	if *path == "" {
		log.Fatal("Must specify file to be interpreted using -f flag.")
	}

	file, err := os.Open(*path)
	if err != nil {
		log.Fatalf("%s: \"%s\"", err, *path)
	}
	in := bufio.NewScanner(file)

	log.SetOutput(os.Stdout)
	var lineScan scanner.Scanner
	for in.Scan() {
		lineScan.Init(strings.NewReader(in.Text()))
		cspParse(&cspLex{s: lineScan})
	}
	file.Close()

	if root != nil {
		print_tree(root)
		dummy := make(chan int)
		go interpret_tree(root, dummy)

		running := forkRunning
		for running != forkDone {
			dummy <- -1
			running = <-dummy
			if running == forkConsumed {
				traceCount++
			}
		}
	}
}

func print_tree(node *cspTree) {
	if node != nil {
		log.Printf("%p, %v", node, *node)
		print_tree(node.left)
		print_tree(node.right)
	}
}

func interpret_tree(node *cspTree, parent chan int) {
	<-parent

	if len(rootTrace) <= traceCount {
		log.Printf("Environment ran out of events.")
		parent <- forkDone
		return
	}
	trace := rootTrace[traceCount]

	switch node.tok {
	case cspParallel:
		left := make(chan int)
		right := make(chan int)
		go interpret_tree(node.left, left)
		go interpret_tree(node.right, right)
		parallelMonitor(left, right)
		parent <- forkDone
	case cspGenChoice, cspOr:
		if node.tok == cspOr || node.left.ident == node.right.ident {
			if rand.Intn(2) == 1 {
				parent <- forkRunning
				interpret_tree(node.right, parent)
			} else {
				parent <- forkRunning
				interpret_tree(node.left, parent)
			}
			break
		}
		fallthrough
	case cspChoice:
		switch {
		case node.left.ident == node.right.ident:
			log.Printf("Cannot have a choice between identical events.")
			parent <- forkDone
		case trace == node.left.ident:
			parent <- forkConsumed
			interpret_tree(node.left.right, parent)
		case trace == node.right.ident:
			parent <- forkConsumed
			interpret_tree(node.right.right, parent)
		default:
			fmt := "Deadlock: environment (%s) " +
				"matches neither of the choice events (%s/%s)"
			log.Printf(fmt, trace, node.left.ident, node.right.ident)
			parent <- forkDone
		}
	case cspEvent:
		switch {
		case trace != node.ident:
			fmt := "Deadlock: environment (%s) " +
				"does not match prefixed event (%s)"
			log.Printf(fmt, trace, node.ident)
			parent <- forkDone
		case node.right != nil:
			parent <- forkConsumed
			interpret_tree(node.right, parent)
		default:
			log.Printf("Process ran out of events.")
			parent <- forkDone
		}
	case cspProcessTok:
		p, ok := processDefinitions[node.ident]
		if ok {
			parent <- forkRunning
			interpret_tree(p, parent)
		} else {
			log.Printf("Process %s is not defined.", node.ident)
			parent <- forkDone
		}
	}
}

func parallelMonitor(left chan int, right chan int) {
	left <- -1
	right <- -1

	var isLeftDone bool
	oneConsumed := false
DoubleMonitor:
	for {
		select {
		case running := <-left:
			switch running {
			case forkRunning:
				left <- -1
			case forkDone:
				isLeftDone = true
				break DoubleMonitor
			case forkConsumed:
				if oneConsumed {
					traceCount++
					oneConsumed = false
					left <- -1
					right <- -1
				} else {
					oneConsumed = true
				}
			}
		case running := <-right:
			switch running {
			case forkRunning:
				right <- -1
			case forkDone:
				isLeftDone = false
				break DoubleMonitor
			case forkConsumed:
				if oneConsumed {
					traceCount++
					oneConsumed = false
					left <- -1
					right <- -1
				} else {
					oneConsumed = true
				}
			}
		}
	}

	var c chan int
	if isLeftDone {
		c = right
	} else {
		c = left
	}

	running := forkRunning
	if oneConsumed {
		traceCount++
	} else {
		running = <-c
	}
	for running != forkDone {
		c <- -1
		running = <-c
		if running == forkConsumed {
			traceCount++
		}
	}
}
