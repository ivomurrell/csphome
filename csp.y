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
	cspEvent
		{
			$$.node = &cspTree{tok: cspEvent}
		}
	| cspProcess
		{
			$$.node = &cspTree{tok: cspProcess}
		}
	| cspEvent cspPrefix Process
		{
			$$.node = &cspTree{tok: cspEvent, right: $3.node}
		}

%%

const eof = 0

type cspLex struct {
	s scanner.Scanner
}

func (x *cspLex) Lex(lvalue *cspSymType) int {
	var tokenType int
	lvalue.ident, tokenType = x.next()
	log.Printf("parsing: %s, %v", lvalue.ident, tokenType)

	return tokenType
}

func (x *cspLex) next() (string, int) {
	var (
		outVal string
		outTok int
	)

	if tok := x.s.Scan(); tok == scanner.Ident {
		outVal = x.s.TokenText()
		if r, _ := utf8.DecodeRuneInString(outVal); unicode.IsUpper(r) {
			outTok = cspProcess
		} else {
			outTok = cspEvent
		}
	} else {
		switch {
		case tok == '-':
			if x.s.Peek() != '>' {
				log.Printf("Unrecognised character: -")
			} else {
				x.s.Next()
				outVal, outTok = "->", cspPrefix
			}
		case tok == '[':
			if x.s.Peek() != ']' {
				log.Printf("Unrecognised character: [")
			} else {
				x.s.Next()
				outVal, outTok = "[]", cspGenChoice
			}
		case tok == '|':
			if x.s.Peek() != '|' {
				outVal, outTok = "|", cspChoice
			} else {
				x.s.Next()
				outVal, outTok = "||", cspParallel
			}
		case tok == scanner.EOF:
			outVal, outTok = "EOF", eof
		default:
			log.Printf("Unrecognised character: %q", tok)
			outVal = string(tok)
		}
	}

	return outVal, outTok
}

func (x *cspLex) Error(s string) {
	log.Printf("parse error: %s", s)
}