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

func errorPass() (err error) {
	for ident, p := range processDefinitions {
		brandProcessEvents(ident, p)

		err = checkAlphabet(p)
		if err != nil {
			return
		}

	}

	return nil
}

func brandProcessEvents(name string, root *cspTree) {
	root.process = name

	if root.left != nil {
		brandProcessEvents(name, root.left)
	}
	if root.right != nil {
		brandProcessEvents(name, root.right)
	}
}

func checkAlphabet(root *cspTree) (err error) {
	if root.tok == cspEvent {
		if !inAlphabet(root.process, root.ident) {
			errFmt := "Syntax error: Event %s not in %s's alphabet."
			return fmt.Errorf(errFmt, root.ident, root.process)
		}
	}

	if root.left != nil {
		err = checkAlphabet(root.left)
		if err != nil {
			return err
		}
	}

	if root.right != nil {
		err = checkAlphabet(root.right)
	}
	return err
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
