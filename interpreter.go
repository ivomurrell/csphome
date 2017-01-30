package main

import (
	"strings"
	"text/scanner"
)

//go:generate go tool yacc -p "csp" -o parser.go csp.y

func main() {
	var s scanner.Scanner
	s.Init(strings.NewReader("aaaa->P"))
	cspParse(&cspLex{s})
}
