package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"text/scanner"
	"time"
)

//go:generate goyacc -p "csp" -o parser.go csp.y

type cspValueMappings map[string]string

var traceCount int = 0

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	path := flag.String("f", "", "File path to CSP definitions.")
	flagUsage := "Use static trees generated at compile time to handle " +
		"channel input. Mirrors the CSP definition more closely whilst " +
		"using significantly more memory."
	useFormalCommunication = flag.Bool("formalchannels", false, flagUsage)
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
	file.Close()
	if wasParserError {
		return
	}

	err = errorPass()

	if err != nil {
		log.Fatal(err)
	} else if rootNode != nil {
		dummy := make(chan bool)
		rootMap := make(cspValueMappings)
		go interpret_tree(rootNode, true, dummy, &rootMap)

		running := true
		for running {
			dummy <- false
			running = <-dummy
			traceCount++
		}

		if len(rootTrace) < traceCount {
			log.Print("Environment ran out of events.")
		} else {
			log.Print("Unexecuted environment events: ",
				rootTrace[traceCount-1:])
		}
	}
}

func print_tree(node *cspTree) {
	if node != nil {
		log.Printf("%p, %v", node, *node)
		print_tree(node.left)
		print_tree(node.right)
	}
}

func interpret_tree(
	node *cspTree,
	needBarrier bool,
	parent chan bool,
	mappings *cspValueMappings) {

	if needBarrier {
		<-parent
	}

	if len(rootTrace) <= traceCount {
		parent <- false
		return
	}
	trace := rootTrace[traceCount]

	switch node.tok {
	case cspParallel:
		left := make(chan bool)
		right := make(chan bool)
		leftMap := *mappings
		rightMap := *mappings

		go interpret_tree(node.left, false, left, &leftMap)
		go interpret_tree(node.right, false, right, &rightMap)

		parallelMonitor(left, right, parent)
	case cspGenChoice, cspOr:
		if node.tok == cspOr || node.left.ident == node.right.ident {
			if rand.Intn(2) == 1 {
				interpret_tree(node.right, false, parent, mappings)
			} else {
				interpret_tree(node.left, false, parent, mappings)
			}
			break
		}
		fallthrough
	case cspChoice:
		if branch, events := choiceTraverse(trace, node); branch != nil {
			interpret_tree(branch, false, parent, mappings)
		} else {
			fmt := "%s: Deadlock: environment (%s) " +
				"matches none of the choice events %v."
			log.Printf(fmt, node.process, trace, events)
			parent <- false
		}
	case cspEvent:
		if node.process != "" && !inAlphabet(node.process, trace) {
			parent <- true
			interpret_tree(node, true, parent, mappings)
		} else {
			if trace != node.ident {
				mappedEvent := (*mappings)[node.ident]

				if trace != mappedEvent {
					fmt := "%s: Deadlock: environment (%s) " +
						"does not match prefixed event (%s)"
					log.Printf(fmt, node.process, trace, node.ident)
					parent <- false
					break
				}
			}

			if node.right == nil {
				log.Printf("%s: Process ran out of events.", node.process)
				parent <- false
				break
			}

			parent <- true
			interpret_tree(node.right, true, parent, mappings)
		}
	case cspProcessTok:
		p, ok := processDefinitions[node.ident]
		if ok {
			interpret_tree(p, false, parent, mappings)
		} else {
			log.Printf("%s: Process %s is not defined.",
				node.process, node.ident)
			parent <- false
		}
	case '!':
		args := strings.Split(trace, ".")
		log.Print("Outputting on ", args[0])
		channels[args[0]] <- args[1]

		parent <- true
		interpret_tree(node.right, true, parent, mappings)
	case '?':
		args := strings.Split(node.ident, ".")
		log.Print("Listening on ", args[0])
		(*mappings)[args[1]] = <-channels[args[0]]

		parent <- true
		interpret_tree(node.right, true, parent, mappings)
	}
}

func parallelMonitor(left chan bool, right chan bool, parent chan bool) {
	var isLeftDone bool
	for {
		if running := <-left; !running {
			isLeftDone = true
			break
		}
		if running := <-right; !running {
			isLeftDone = false
			break
		}

		parent <- true
		<-parent

		left <- true
		right <- true
	}

	var c chan bool
	running := true
	if isLeftDone {
		c = right
		running = <-c
	} else {
		c = left
	}
	parent <- running

	for running {
		<-parent

		c <- true
		running = <-c

		parent <- running
	}
}

func choiceTraverse(target string, root *cspTree) (*cspTree, []string) {
	switch root.tok {
	case cspEvent:
		if root.ident == target {
			return root, []string{root.ident}
		} else {
			return nil, []string{root.ident}
		}
	case cspChoice:
		result, leftEvents := choiceTraverse(target, root.left)
		if result != nil {
			return result, leftEvents
		}

		result, rightEvents := choiceTraverse(target, root.right)
		return result, append(leftEvents, rightEvents...)
	default:
		log.Printf("Mixing a choice operator with a %v is not supported",
			root.tok)
		return nil, nil
	}
}

func errorPass() error {
	for ident, p := range processDefinitions {
		err := errorPassProcess(ident, p)
		if err != nil {
			return err
		}
	}

	return nil
}

func errorPassProcess(name string, root *cspTree) (err error) {
	brandProcessEvents(name, root)

	err = checkAlphabet(root)
	if err != nil {
		return
	}

	err = checkDeterministicChoice(root)
	if err != nil {
		return
	}

	if root.left != nil {
		err = errorPassProcess(name, root.left)
		if err != nil {
			return
		}
	}

	if root.right != nil {
		err = errorPassProcess(name, root.right)
	}
	return
}

func brandProcessEvents(name string, root *cspTree) {
	root.process = name
}

func checkAlphabet(root *cspTree) error {
	if root.tok == cspEvent {
		if !inAlphabet(root.process, root.ident) {
			errFmt := "Syntax error: Event %s not in %s's alphabet."
			return fmt.Errorf(errFmt, root.ident, root.process)
		}
	} else if root.tok == '?' {
		i := strings.LastIndex(root.ident, ".") + 1
		variable := root.ident[i:]
		if !inAlphabet(root.process, variable) {
			alphabet := alphabets[root.process]
			alphabets[root.process] = append(alphabet, variable)
		}
	}

	return nil
}

func checkDeterministicChoice(root *cspTree) error {
	if root.tok == cspChoice {
		var left, right string
		switch root.left.tok {
		case cspEvent:
			left = root.left.ident
		case cspProcessTok:
			left = processDefinitions[root.left.process].ident
		default:
			log.Fatal("Do not currently support multiple choice branches.")
		}
		switch root.right.tok {
		case cspEvent:
			right = root.right.ident
		case cspProcessTok:
			right = processDefinitions[root.right.process].ident
		default:
			log.Fatal("Do not currently support multiple choice branches.")
		}

		if left == right {
			errFmt := "Syntax error: Cannot have a choice " +
				"between identical events (%s + %s)."
			return fmt.Errorf(errFmt, left, right)
		}
	}

	return nil
}

func inAlphabet(process string, event string) (found bool) {
	alphabet := alphabets[process]

	for _, a := range alphabet {
		if a == event {
			found = true
			break
		}
	}

	return
}
