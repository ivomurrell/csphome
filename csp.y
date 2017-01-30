%{

package main

import (
	"log"
)

%}

%union {
	val int
}

%token cspEvent cspProcess cspChoice cspParallel
%left cspPrefix

%%

start: ;

%%

const eof = 0

type cspLex string

func (x *cspLex) Lex(yyl *cspSymType) int {
	log.Printf("parsing: %v", *x)
	return yyl.val
}

func (x *cspLex) Error(s string) {
	log.Printf("parse error: %s", s)
}