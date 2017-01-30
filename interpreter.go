package main

//go:generate go tool yacc -p "csp" -o parser.go csp.y

func main() {
	x := cspLex("a -> P")
	cspParse(&x)
}
