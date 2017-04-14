package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"text/scanner"
	"time"
)

//go:generate goyacc -p "csp" -o parser.go csp.y

type cspValueMappings map[string]string

type cspChannel struct {
	blockedEvents []string
	needToBlock   bool
	traceCount    int
	c             chan bool
	isOpen        bool
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	path := flag.String("f", "", "File path to CSP definitions.")
	flagUsage := "Use static trees generated at compile time to handle " +
		"channel input. Mirrors the CSP definition more closely whilst " +
		"using significantly more memory."
	useFormalCommunication = *flag.Bool("formalchannels", false, flagUsage)
	flag.Parse()

	if *path == "" {
		log.Fatal("Must specify file to be interpreted using -f flag.")
	}

	interpretTree(*path)
}

func printTree(node *cspTree) {
	if node != nil {
		log.Printf("%p, %v", node, *node)
		for i := 0; i < len(node.branches); i++ {
			printTree(node.branches[i])
		}
	}
}

func interpretTree(path string) cspEventList {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("%s: \"%s\"", err, path)
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
		return nil
	}

	err = errorPass()

	if err != nil {
		log.Fatal(err)
	} else if rootNode != nil {
		dummy := cspChannel{nil, true, 0, make(chan bool), true}
		rootMap := make(cspValueMappings)
		go traverseTree(rootNode, &dummy, rootMap)

		running := true
		for running {
			dummy.c <- false
			running = <-dummy.c
		}

		if len(rootTrace) < dummy.traceCount {
			log.Print("Environment ran out of events.")
			return nil
		} else {
			remainingEvents := rootTrace[dummy.traceCount-1:]
			log.Print("Unexecuted environment events: ", remainingEvents)
			return remainingEvents
		}
	}

	return nil
}

func traverseTree(
	node *cspTree,
	parent *cspChannel,
	mappings cspValueMappings) {

	if parent.needToBlock {
		<-parent.c
		parent.needToBlock = false
	}

	if len(rootTrace) <= parent.traceCount {
		terminateProcess(parent)
		return
	}
	trace := rootTrace[parent.traceCount]

	switch node.tok {
	case cspParallel:
		var blockedEvents []string
		if parent.blockedEvents != nil {
			blockedEvents = parent.blockedEvents
		} else {
			blockedEvents = getConjunctEvents(node)
		}

		localChans := make([]*cspChannel, len(node.branches))
		for i := 0; i < len(node.branches); i++ {
			localChans[i] = &cspChannel{
				blockedEvents, false, parent.traceCount, make(chan bool), true}
			newMap := make(cspValueMappings)
			for k, v := range mappings {
				newMap[k] = v
			}

			go traverseTree(node.branches[i], localChans[i], newMap)
		}

		parallelMonitor(localChans, parent)
	case cspOr:
		i := rand.Intn(len(node.branches))
		traverseTree(node.branches[i], parent, mappings)
	case cspChoice:
		if branch, events := choiceTraverse(trace, node); branch != nil {
			traverseTree(branch, parent, mappings)
		} else if !inAlphabet(node.process, trace) {
			consumeEvent(parent)
			traverseTree(node, parent, mappings)
		} else {
			fmtStr := "%s: Deadlock: environment (%s) " +
				"matches none of the choice events %v."
			log.Printf(fmtStr, node.process, trace, events)
			terminateProcess(parent)
		}
	case cspGenChoice:
		if branches, events := genChoiceTraverse(trace, node); branches != nil {
			bIndex := rand.Intn(len(branches))
			traverseTree(branches[bIndex], parent, mappings)
		} else if !inAlphabet(node.process, trace) {
			consumeEvent(parent)
			traverseTree(node, parent, mappings)
		} else {
			fmtStr := "%s: Deadlock: environment (%s) " +
				"matches none of the general choice events %v."
			log.Printf(fmtStr, node.process, trace, events)
			terminateProcess(parent)
		}
	case cspEvent:
		if !inAlphabet(node.process, trace) {
			consumeEvent(parent)
			traverseTree(node, parent, mappings)
		} else {
			if trace != node.ident {
				mappedEvent := mappings[node.ident]

				if trace != mappedEvent {
					fmtStr := "%s: Deadlock: environment (%s) " +
						"does not match prefixed event (%s)"
					log.Printf(fmtStr, node.process, trace, node.ident)
					terminateProcess(parent)
					break
				}
			}

			if node.branches == nil {
				log.Printf("%s: Process ran out of events.", node.process)

				if parent != nil {
					parent.c <- true
					<-parent.c
					parent.c <- false
				}
				break
			}

			consumeEvent(parent)
			traverseTree(node.branches[0], parent, mappings)
		}
	case cspProcessTok:
		p, ok := processDefinitions[node.ident]
		if ok {
			traverseTree(p, parent, mappings)
		} else {
			log.Printf("%s: Process %s is not defined.",
				node.process, node.ident)
			terminateProcess(parent)
		}
	case '!':
		args := strings.Split(trace, ".")
		if len(args) != 2 {
			fmtStr := "%s: Deadlock: Expected output event but " +
				"instead found event %s."
			log.Printf(fmtStr, node.process, trace)
			terminateProcess(parent)
			break
		}
		channels[args[0]] <- args[1]

		consumeEvent(parent)
		traverseTree(node.branches[0], parent, mappings)
	case '?':
		args := strings.Split(node.ident, ".")
		mappings[args[1]] = <-channels[args[0]]

		consumeEvent(parent)
		traverseTree(node.branches[0], parent, mappings)
	default:
		log.Printf("Unrecognised token %v.", node.tok)
		terminateProcess(parent)
	}
}

func consumeEvent(parent *cspChannel) {
	if parent != nil {
		event := rootTrace[parent.traceCount]

		for _, blockedEvent := range parent.blockedEvents {
			if event == blockedEvent {
				parent.c <- true
				parent.needToBlock = true

				break
			}
		}

		parent.traceCount++
	}
}

func terminateProcess(parent *cspChannel) {
	if parent != nil {
		parent.c <- false
	}
}

func parallelMonitor(branches []*cspChannel, parent *cspChannel) {
	stillRunning := len(branches)
	for {
		for _, branch := range branches {
			if !branch.isOpen {
				continue
			}
			if running := <-branch.c; !running {
				stillRunning--
				branch.isOpen = false
			}
		}

		if stillRunning > 0 {
			parent.c <- true
			<-parent.c
		} else {
			break
		}

		for _, branch := range branches {
			if branch.isOpen {
				branch.c <- true
			}
		}
	}

	for _, branch := range branches {
		if branch.traceCount >= parent.traceCount {
			parent.traceCount = branch.traceCount + 1
		}
	}
	parent.c <- false
}

func getConjunctEvents(root *cspTree) []string {
	events := make([]cspEventList, 0, len(root.branches))
	for i := 0; i < len(root.branches); i++ {
		gatheredEvents := gatherEvents(root.branches[i])
		sort.Strings(gatheredEvents)
		events = append(events, gatheredEvents)
	}

	conjunct := make(cspEventList, 0)
	for ei, e := range events {
		for _, event := range e {
			ci := sort.SearchStrings(conjunct, event)
			if ci < len(conjunct) && conjunct[ci] == event {
				continue
			}

			isExclusive := true
			for oi, other := range events {
				if oi == ei {
					continue
				}
				i := sort.SearchStrings(other, event)
				if i < len(other) && other[i] == event {
					isExclusive = false
					break
				}
			}

			if !isExclusive {
				if ci >= len(conjunct) {
					conjunct = append(conjunct, event)
				} else {
					conjunct = append(conjunct[:ci],
						append(cspEventList{event}, conjunct[ci:]...)...)
				}
			}
		}
	}

	return conjunct
}

func gatherEvents(root *cspTree) []string {
	switch root.tok {
	case cspEvent:
		events := gatherEvents(root.branches[0])
		for _, event := range events {
			if root.ident == event {
				return events
			}
		}
		return append(events, root.ident)
	case cspProcessTok:
		return alphabets[root.ident]
	case cspChoice, cspGenChoice, cspOr, cspParallel:
		var events []string
		for i := 0; i < len(root.branches); i++ {
			events = append(events, gatherEvents(root.branches[i])...)
		}
		sort.Strings(events)
		if events == nil {
			return nil
		}

		uniqueEvents := make([]string, 1, len(events))
		uniqueEvents[0] = events[0]
		for i, event := range events[1:] {
			if event != events[i] {
				uniqueEvents = append(uniqueEvents, event)
			}
		}

		return uniqueEvents
	}

	return nil
}

func choiceTraverse(target string, root *cspTree) (*cspTree, []string) {
	switch root.tok {
	case cspEvent:
		if root.ident == target {
			return root, []string{root.ident}
		} else {
			return nil, []string{root.ident}
		}
	case cspProcessTok:
		return choiceTraverse(target, processDefinitions[root.ident])
	case cspChoice:
		var events []string
		for i := 0; i < len(root.branches); i++ {
			result, newEvents := choiceTraverse(target, root.branches[i])
			events = append(events, newEvents...)
			if result != nil {
				return result, events
			}
		}
		return nil, events
	case cspGenChoice:
		results, events := genChoiceTraverse(target, root)
		if len(results) > 1 {
			log.Print("Cannot mix a choice with a general choice degenerated " +
				"to nondeterminism.")
			return nil, nil
		} else {
			return results[0], events
		}
	default:
		log.Printf("Mixing a choice operator with a %v is not supported",
			root.tok)
		return nil, nil
	}
}

func genChoiceTraverse(target string, root *cspTree) ([]*cspTree, []string) {
	switch root.tok {
	case cspEvent:
		if root.ident == target {
			return []*cspTree{root}, []string{root.ident}
		} else {
			return nil, []string{root.ident}
		}
	case cspProcessTok:
		return genChoiceTraverse(target, processDefinitions[root.ident])
	case cspChoice:
		branch, events := choiceTraverse(target, root)
		return []*cspTree{branch}, events
	case cspGenChoice:
		var (
			branches []*cspTree
			events   []string
		)
		for i := 0; i < len(root.branches); i++ {
			newBranches, newEvents :=
				genChoiceTraverse(target, root.branches[i])
			branches = append(branches, newBranches...)
			events = append(events, newEvents...)
		}

		return branches, events
	default:
		fmtStr := "Mixing a general choice operator with " +
			"a %v is not supported"
		log.Printf(fmtStr, root.tok)
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

	return errorPassProcess("", rootNode)
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

	for i := 0; i < len(root.branches); i++ {
		err = errorPassProcess(name, root.branches[i])
		if err != nil {
			return
		}
	}

	return
}

func brandProcessEvents(name string, root *cspTree) {
	root.process = name
}

func checkAlphabet(root *cspTree) error {
	switch root.tok {
	case cspEvent:
		if !inAlphabet(root.process, root.ident) {
			errFmt := "Syntax error: Event %s not in %s's alphabet."
			return fmt.Errorf(errFmt, root.ident, root.process)
		}
	case cspProcessTok:
		alphabets[root.process] =
			append(alphabets[root.process], alphabets[root.ident]...)
	case '?':
		i := strings.LastIndex(root.ident, ".") + 1
		variable := root.ident[i:]
		if !inAlphabet(root.process, variable) {
			alphabet := alphabets[root.process]
			alphabets[root.process] = append(alphabet, variable)
		}

		channel := root.ident[:i-1]
		for _, cEvent := range channelAlphas[channel] {
			if !inAlphabet(root.process, cEvent) {
				errFmt := "Syntax error: %s's alphabet is not a superset of " +
					"channel %s's alphabet."
				return fmt.Errorf(errFmt, root.process, channel)
			}
		}
	}

	return nil
}

func checkDeterministicChoice(root *cspTree) error {
	if root.tok == cspChoice {
		for i := 0; i < len(root.branches); i++ {
			var source string
			sourceNode := root.branches[i]
			switch sourceNode.tok {
			case cspEvent:
				source = sourceNode.ident
			case cspProcessTok:
				source = processDefinitions[sourceNode.ident].ident
			}
			for j := 0; j < len(root.branches); j++ {
				if i == j {
					continue
				}

				var target string
				targetNode := root.branches[j]
				switch targetNode.tok {
				case cspEvent:
					target = targetNode.ident
				case cspProcessTok:
					target = processDefinitions[targetNode.ident].ident
				}

				if source == target {
					errFmt := "Syntax error: Cannot have a choice " +
						"between identical events (%s + %s)."
					return fmt.Errorf(errFmt, source, target)
				}
			}
		}
	}

	return nil
}

func inAlphabet(process, event string) bool {
	if process == "" {
		return true
	} else {
		alphabet := alphabets[process]
		found := false

		for _, a := range alphabet {
			if a == event {
				found = true
				break
			}
		}

		return found
	}
}
