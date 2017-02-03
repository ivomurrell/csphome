package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
	"text/scanner"
)

//go:generate go tool yacc -p "csp" -o parser.go csp.y

var envCount int = 0

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
	switch node.tok {
	case cspGenChoice:
		if node.left.ident == node.right.ident {
			// Simulation of non-determinism arbitrarily picks left sequence.
			interpret_tree(node.left)
			break
		}
		fallthrough
	case cspChoice:
		switch {
		case len(env) <= envCount:
			log.Printf("Environment ran out of events.")
		case node.left.ident == node.right.ident:
			log.Printf("Cannot have a choice between identical events.")
		case env[envCount] == node.left.ident:
			envCount++
			interpret_tree(node.left.right)
		case env[envCount] == node.right.ident:
			envCount++
			interpret_tree(node.right.right)
		default:
			fmt := "Deadlock: environment (%s) " +
				"matches neither of the choice events (%s/%s)"
			log.Printf(fmt, env[envCount], node.left.ident, node.right.ident)
		}
	case cspEvent:
		switch {
		case len(env) <= envCount:
			log.Printf("Environment ran out of events.")
		case env[envCount] != node.ident:
			fmt := "Deadlock: environment (%s) " +
				"does not match prefixed event (%s)"
			log.Printf(fmt, env[envCount], node.ident)
		case node.right != nil:
			envCount++
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
