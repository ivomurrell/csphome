%{

package main

import (
	"log"
	"text/scanner"
	"unicode"
	"unicode/utf8"
)

%}

%union {
	ident string
}

%token cspEvent cspProcess cspChoice cspParallel
%left cspPrefix

%%

start: expr;

expr: cspEvent cspPrefix cspProcess;

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
			x.s.Scan()
			if x.s.TokenText() != ">" {
				log.Printf("Unrecognised character: -")
			}
			outVal, outTok = "->", cspPrefix
		case tok == '[':
			x.s.Scan()
			if x.s.TokenText() != "]" {
				log.Printf("Unrecognised character: [")
			}
			outVal, outTok = "[]", cspChoice
		case tok == '|':
			x.s.Scan()
			if x.s.TokenText() != "|" {
				log.Printf("Unrecognised character: |")
			}
			outVal, outTok = "||", cspParallel
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