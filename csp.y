%{

package main

import (
	"log"
	"text/scanner"
	"unicode"
	"unicode/utf8"
)

type cspTree struct {
	tok int
	ident string
	left *cspTree
	right *cspTree
}

var root *cspTree

%}

%union {
	node *cspTree
	tok int
}

%token cspEvent cspProcess cspChoice cspGenChoice cspParallel
%left cspPrefix

%%

Start:
	Expr
		{
			root = $1.node
			$$ = $1
		}

Expr:
	Process {$$ = $1}
	| Process Choice Expr
		{
			$$.node = &cspTree{tok: $2.tok, left: $1.node, right: $3.node}
		}

Choice:
	cspChoice {$$.tok = cspChoice}
	| cspGenChoice {$$.tok = cspGenChoice}
	| cspParallel {$$.tok = cspParallel}

Process:
	cspEvent {$$ = $1}
	| cspProcess {$$ = $1}
	| cspEvent cspPrefix Process
		{
			$1.node.right = $3.node
			$$ = $1
		}

%%

const eof = 0

type cspLex struct {
	s scanner.Scanner
}

func (x *cspLex) Lex(lvalue *cspSymType) int {
	var token int

	if t := x.s.Scan(); t == scanner.Ident {
		ident := x.s.TokenText()
		if r, _ := utf8.DecodeRuneInString(ident); unicode.IsUpper(r) {
			token = cspProcess
		} else {
			token = cspEvent
		}
		lvalue.node = &cspTree{tok: token, ident: ident}
	} else {
		switch {
		case t == '-':
			if x.s.Peek() != '>' {
				log.Printf("Unrecognised character: -")
			} else {
				x.s.Next()
				token = cspPrefix
			}
		case t == '[':
			if x.s.Peek() != ']' {
				log.Printf("Unrecognised character: [")
			} else {
				x.s.Next()
				token = cspGenChoice
			}
		case t == '|':
			if x.s.Peek() != '|' {
				token = cspChoice
			} else {
				x.s.Next()
				token = cspParallel
			}
		case t == scanner.EOF:
			token = eof
		default:
			log.Printf("Unrecognised character: %q", t)
		}
	}

	return token
}

func (x *cspLex) Error(s string) {
	log.Printf("parse error: %s", s)
}