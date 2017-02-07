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
	process string
	left *cspTree
	right *cspTree
}

type cspEventList []string
type cspAlphabetMap map[string]cspEventList

var root *cspTree
var rootTrace cspEventList

var processDefinitions map[string]*cspTree = make(map[string]*cspTree)
var alphabets cspAlphabetMap = make(cspAlphabetMap)

var eventBuf cspEventList

%}

%union {
	node *cspTree
	ident string
}

%type <node> Expr Process

%token <ident> cspEvent cspProcessTok
%token cspLet cspAlphabetTok cspTraceDef
%left cspParallel
%left cspGenChoice cspOr
%left cspChoice
%right cspPrefix

%%

Start:
	Expr {root = $1}
	| Decl
	|

Expr:
	Process {$$ = $1}
	| Expr cspChoice Expr
		{
			$$ = &cspTree{tok: cspChoice, left: $1, right: $3}
		}
	| Expr cspGenChoice Expr
		{
			$$ = &cspTree{tok: cspGenChoice, left: $1, right: $3}
		}
	| Expr cspOr Expr
		{
			$$ = &cspTree{tok: cspOr, left: $1, right: $3}
		}
	| Expr cspParallel Expr
		{
			$$ = &cspTree{tok: cspParallel, left: $1, right: $3}
		}

Process:
	cspEvent {$$ = &cspTree{tok: cspEvent, ident: $1}}
	| cspProcessTok {$$ = &cspTree{tok: cspProcessTok, ident: $1}}
	| cspEvent cspPrefix Process
		{
			$$ = &cspTree{tok: cspEvent, ident: $1, right: $3}
		}

Decl:
	cspLet cspAlphabetTok cspProcessTok '=' EventSet
		{
			alphabets[$3] = eventBuf
			eventBuf = nil
		}
	| cspTraceDef EventSet
		{
			rootTrace = eventBuf
			eventBuf = nil
		}
	| cspLet cspProcessTok '=' Expr {processDefinitions[$2] = $4}

EventSet:
	cspEvent {eventBuf = append(eventBuf, $1)}
	| EventSet cspEvent {eventBuf = append(eventBuf, $2)}
	| EventSet ',' cspEvent {eventBuf = append(eventBuf, $3)}

%%

const eof = 0

type cspLex struct {
	s scanner.Scanner
}

func (x *cspLex) Lex(lvalue *cspSymType) (token int) {
	if t := x.peekNextSymbol(); t == 'α' {
		x.s.Next()
		token = cspAlphabetTok
	} else if t = x.s.Scan(); t == scanner.Ident {
		ident := x.s.TokenText()
		switch ident {
		case "let":
			token = cspLet
		case "tracedef":
			token = cspTraceDef
		default:
			r, _ := utf8.DecodeRuneInString(ident)
			if unicode.IsUpper(r) {
				token = cspProcessTok
			} else {
				token = cspEvent
			}
		}
		lvalue.ident = ident
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
			switch x.s.Peek() {
			case '|':
				x.s.Next()
				if x.s.Peek() != ']' {
					log.Printf("Unrecognised sequence: \"[|\"")
				} else {
					x.s.Next()
					token = cspGenChoice
				}
			case ']':
				x.s.Next()
				token = cspOr
			default:
				log.Printf("Unrecognised character: [")
			}
		case '|':
			if x.s.Peek() != '|' {
				token = cspChoice
			} else {
				x.s.Next()
				token = cspParallel
			}
		case scanner.EOF:
			token = eof
		case '=', ',':
			token = int(t)
		default:
			log.Printf("Unrecognised character: %q", t)
		}
	}

	return
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