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

type cspAlphabet []string
type cspAlphabetMap map[string]cspAlphabet

var root *cspTree

var alphabets cspAlphabetMap = make(cspAlphabetMap)
var alphaBuf cspAlphabet

%}

%union {
	node *cspTree
	tok int
}

%type <node> Expr Process

%token <node> cspEvent cspProcess
%token cspLet cspAlphabetTok
%left <tok> cspParallel
%left <tok> cspGenChoice
%left <tok> cspChoice
%left cspPrefix

%%

Start:
	Expr {root = $1}
	| Decl

Expr:
	Process {$$ = $1}
	| Expr cspChoice Expr {$$ = &cspTree{tok: $2, left: $1, right: $3}}
	| Expr cspGenChoice Expr {$$ = &cspTree{tok: $2, left: $1, right: $3}}
	| Expr cspParallel Expr {$$ = &cspTree{tok: $2, left: $1, right: $3}}

Process:
	cspEvent {$$ = $1}
	| cspProcess {$$ = $1}
	| cspEvent cspPrefix Process
		{
			$1.right = $3
			$$ = $1
		}

Decl:
	cspLet cspAlphabetTok cspProcess '=' EventSet
		{
			alphabets[$3.ident] = alphaBuf
			alphaBuf = nil
		}

EventSet:
	cspEvent {alphaBuf = append(alphaBuf, $1.ident)}
	| EventSet cspEvent {alphaBuf = append(alphaBuf, $2.ident)}
	| EventSet ',' cspEvent {alphaBuf = append(alphaBuf, $3.ident)}

%%

const eof = 0

type cspLex struct {
	s scanner.Scanner
}

func (x *cspLex) Lex(lvalue *cspSymType) int {
	var token int

	if t := x.peekNextSymbol(); t == 'α' {
		x.s.Next()
		token = cspAlphabetTok
	} else if t = x.s.Scan(); t == scanner.Ident {
		ident := x.s.TokenText()
		switch {
		case ident == "let":
			token = cspLet
		default:
			r, _ := utf8.DecodeRuneInString(ident)
			if unicode.IsUpper(r) {
				token = cspProcess
			} else {
				token = cspEvent
			}
		}
		lvalue.node = &cspTree{tok: token, ident: ident}
	} else {
		switch t {
		case '-':
			if x.s.Peek() != '>' {
				log.Printf("Unrecognised character: -")
			} else {
				x.s.Next()
				token = cspPrefix
			}
		case '[':
			if x.s.Peek() != ']' {
				log.Printf("Unrecognised character: [")
			} else {
				x.s.Next()
				token = cspGenChoice
				lvalue.tok = token
			}
		case '|':
			if x.s.Peek() != '|' {
				token = cspChoice
			} else {
				x.s.Next()
				token = cspParallel
			}
			lvalue.tok = token
		case scanner.EOF:
			token = eof
		case '=', ',':
			token = int(t)
		default:
			log.Printf("Unrecognised character: %q", t)
		}
	}

	return token
}

func (x *cspLex) peekNextSymbol() rune {
	for {
		s := x.s.Peek()
		if unicode.IsSpace(s) {
			x.s.Next()
		} else {
			return s
		}
	}
}

func (x *cspLex) Error(s string) {
	log.Printf("parse error: %s", s)
}