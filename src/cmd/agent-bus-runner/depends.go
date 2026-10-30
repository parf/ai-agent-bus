package main

import (
	"fmt"
	"strings"
)

// orderDepends puts each agent after the ones its `depends` names, so a
// start loop that walks the list starts them in that order
// (Plans/R1.0-Release/runner-implementation.md#what-it-comes-after). It does
// nothing else: it waits for no readiness and watches nothing.
//
// An entry that names nothing installed here refuses the agent, and a cycle
// refuses every agent on it with the cycle named: a typo or a loop left alone
// is an agent that starts in the wrong order and looks healthy doing it. Among
// agents that need no order the list keeps its order.
func orderDepends(list []installed) []installed {
	byKey := map[string]int{}
	for i, in := range list {
		byKey[in.Key] = i
	}
	// An entry is a bus name, `#key`, naming that agent or instance, or an
	// agent whose instances are all meant.
	resolve := func(entry string) []int {
		name := strings.TrimPrefix(entry, "#")
		if at := strings.IndexByte(name, '@'); at >= 0 {
			name = name[:at]
		}
		var found []int
		for i, in := range list {
			if in.Key == name || strings.HasPrefix(in.Key, name+"/") {
				found = append(found, i)
			}
		}
		return found
	}
	deps := make([][]int, len(list))
	for i := range list {
		in := &list[i]
		if in.Refused != "" {
			continue
		}
		for _, entry := range in.Agent.Depends {
			found := resolve(entry)
			if len(found) == 0 {
				in.Refused = fmt.Sprintf("agent.json: depends names %s, which is not installed here; a host that runs it elsewhere removes the entry in runner/%s/agent.json", entry, in.Dir)
				deps[i] = nil
				break
			}
			deps[i] = append(deps[i], found...)
		}
	}
	const (
		unseen = iota
		onPath
		done
	)
	state := make([]int, len(list))
	var path []int
	var order []int
	var visit func(i int)
	visit = func(i int) {
		switch state[i] {
		case done:
			return
		case onPath:
			start := 0
			for path[start] != i {
				start++
			}
			cycle := append(append([]int{}, path[start:]...), i)
			names := make([]string, len(cycle))
			for n, c := range cycle {
				names[n] = "#" + list[c].Key
			}
			for _, c := range cycle[:len(cycle)-1] {
				list[c].Refused = "agent.json: depends makes a cycle: " + strings.Join(names, " -> ")
			}
			return
		}
		state[i] = onPath
		path = append(path, i)
		for _, d := range deps[i] {
			visit(d)
		}
		path = path[:len(path)-1]
		state[i] = done
		order = append(order, i)
	}
	for i := range list {
		visit(i)
	}
	out := make([]installed, 0, len(list))
	for _, i := range order {
		out = append(out, list[i])
	}
	return out
}
