package main

import (
	"bufio"
	"flag"
	"fmt"
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

	err = errorPass()

	if err != nil {
		log.Fatal(err)
	} else if root != nil {
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
		case !inAlphabet(node.process, node.ident):
			parent <- true
			interpret_tree(node, true, parent)
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
		p, ok := processDefinitions[node.process]
		if ok {
			interpret_tree(p, false, parent)
		} else {
			log.Printf("Process %s is not defined.", node.process)
			parent <- false
		}
	}
}

func parallelMonitor(left chan bool, right chan bool) {
	var isLeftDone bool
	for {
		if running := <-left; !running {
			isLeftDone = true
			break
		}
		if running := <-right; !running {
			isLeftDone = false
			break
		}
		traceCount++
		left <- true
		right <- true
	}

	var c chan bool
	running := true
	if isLeftDone {
		c = right
		running = <-c
	} else {
		c = left
	}
	traceCount++

	for running {
		c <- true
		running = <-c
		traceCount++
	}
}

func errorPass() error {
	for ident, p := range processDefinitions {
		err := errorPassProcess(ident, p)
		if err != nil {
			return err
		}
	}

	return nil
}

func errorPassProcess(name string, root *cspTree) (err error) {
	brandProcessEvents(name, root)

	err = checkAlphabet(root)
	if err != nil {
		return
	}

	if root.left != nil {
		err = errorPassProcess(name, root.left)
		if err != nil {
			return
		}
	}

	if root.right != nil {
		err = errorPassProcess(name, root.right)
	}
	return
}

func brandProcessEvents(name string, root *cspTree) {
	root.process = name
}

func checkAlphabet(root *cspTree) error {
	if root.tok == cspEvent {
		if !inAlphabet(root.process, root.ident) {
			errFmt := "Syntax error: Event %s not in %s's alphabet."
			return fmt.Errorf(errFmt, root.ident, root.process)
		}
	}

	return nil
}

func inAlphabet(process string, event string) (found bool) {
	alphabet := alphabets[process]

	for _, a := range alphabet {
		if a == event {
			found = true
			break
		}
	}

	return
}
