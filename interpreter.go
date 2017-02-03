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

	if root != nil {
		print_tree(root)
		interpret_tree(root)
	}
}

func print_tree(node *cspTree) {
	if node != nil {
		log.Printf("%p, %v", node, *node)
		print_tree(node.left)
		print_tree(node.right)
	}
}

func interpret_tree(node *cspTree) {
	if len(rootTrace) <= traceCount {
		log.Printf("Environment ran out of events.")
		return
	}
	trace := rootTrace[traceCount]

	switch node.tok {
	case cspGenChoice, cspOr:
		if node.tok == cspOr || node.left.ident == node.right.ident {
			if rand.Intn(2) == 1 {
				interpret_tree(node.right)
			} else {
				interpret_tree(node.left)
			}
			break
		}
		fallthrough
	case cspChoice:
		switch {
		case node.left.ident == node.right.ident:
			log.Printf("Cannot have a choice between identical events.")
		case trace == node.left.ident:
			traceCount++
			interpret_tree(node.left.right)
		case trace == node.right.ident:
			traceCount++
			interpret_tree(node.right.right)
		default:
			fmt := "Deadlock: environment (%s) " +
				"matches neither of the choice events (%s/%s)"
			log.Printf(fmt, trace, node.left.ident, node.right.ident)
		}
	case cspEvent:
		switch {
		case trace != node.ident:
			fmt := "Deadlock: environment (%s) " +
				"does not match prefixed event (%s)"
			log.Printf(fmt, trace, node.ident)
		case node.right != nil:
			traceCount++
			interpret_tree(node.right)
		default:
			log.Printf("Process ran out of events.")
		}
	case cspProcessTok:
		p, ok := processDefinitions[node.ident]
		if ok {
			interpret_tree(p)
		} else {
			log.Printf("Process %s is not defined.", node.ident)
		}
	}
}
