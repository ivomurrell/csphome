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
		dummy := make(chan bool)
		go interpret_tree(root, true, dummy)

		running := true
		for running {
			dummy <- false
			running = <-dummy
			traceCount++
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

func interpret_tree(node *cspTree, needBarrier bool, parent chan bool) {
	if needBarrier {
		<-parent
	}

	if len(rootTrace) <= traceCount {
		log.Printf("Environment ran out of events.")
		parent <- false
		return
	}
	trace := rootTrace[traceCount]

	switch node.tok {
	case cspParallel:
		left := make(chan bool)
		right := make(chan bool)
		go interpret_tree(node.left, false, left)
		go interpret_tree(node.right, false, right)
		parallelMonitor(left, right)
		parent <- false
	case cspGenChoice, cspOr:
		if node.tok == cspOr || node.left.ident == node.right.ident {
			if rand.Intn(2) == 1 {
				interpret_tree(node.right, false, parent)
			} else {
				interpret_tree(node.left, false, parent)
			}
			break
		}
		fallthrough
	case cspChoice:
		switch {
		case node.left.ident == node.right.ident:
			log.Printf("Cannot have a choice between identical events.")
			parent <- false
		case trace == node.left.ident:
			parent <- true
			interpret_tree(node.left.right, true, parent)
		case trace == node.right.ident:
			parent <- true
			interpret_tree(node.right.right, true, parent)
		default:
			fmt := "Deadlock: environment (%s) " +
				"matches neither of the choice events (%s/%s)"
			log.Printf(fmt, trace, node.left.ident, node.right.ident)
			parent <- false
		}
	case cspEvent:
		switch {
		case trace != node.ident:
			fmt := "Deadlock: environment (%s) " +
				"does not match prefixed event (%s)"
			log.Printf(fmt, trace, node.ident)
			parent <- false
		case node.right != nil:
			parent <- true
			interpret_tree(node.right, true, parent)
		default:
			log.Printf("Process ran out of events.")
			parent <- false
		}
	case cspProcessTok:
		p, ok := processDefinitions[node.ident]
		if ok {
			interpret_tree(p, false, parent)
		} else {
			log.Printf("Process %s is not defined.", node.ident)
			parent <- false
		}
	}
}

func parallelMonitor(left chan bool, right chan bool) {
	var isLeftDone bool
	oneConsumed := false
DoubleMonitor:
	for {
		select {
		case running := <-left:
			if running {
				if oneConsumed {
					traceCount++
					oneConsumed = false
					left <- true
					right <- true
				} else {
					oneConsumed = true
				}
			} else {
				isLeftDone = true
				break DoubleMonitor
			}
		case running := <-right:
			if running {
				if oneConsumed {
					traceCount++
					oneConsumed = false
					left <- true
					right <- true
				} else {
					oneConsumed = true
				}
			} else {
				isLeftDone = false
				break DoubleMonitor
			}
		}
	}

	var c chan bool
	if isLeftDone {
		c = right
	} else {
		c = left
	}

	running := true
	if oneConsumed {
		traceCount++
	} else {
		running = <-c
	}
	for running {
		c <- true
		running = <-c
		traceCount++
	}
}
