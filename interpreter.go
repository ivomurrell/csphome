package main

import (
	"log"
	"strings"
	"text/scanner"
)

//go:generate go tool yacc -p "csp" -o parser.go csp.y

func main() {
	var s scanner.Scanner
	s.Init(strings.NewReader("aaaa->Q|b->c->P"))
	cspParse(&cspLex{s})

	print_tree(root)
}

func print_tree(node *cspTree) {
	if node != nil {
		log.Printf("%p, %v", node, *node)
		print_tree(node.left)
		print_tree(node.right)
	}
}
