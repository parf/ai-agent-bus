package main

import (
	"fmt"
	"strings"
)

// The ask is a short health verdict; the status method keeps the full observation.
func statusLine(st statusAnswer) string {
	if st.Error != "" {
		return "failed: no agents loaded"
	}
	kept, running, healthy := 0, 0, 0
	var demand, bad []string
	for _, a := range st.Agents {
		if !a.Enabled {
			continue
		}
		label := a.Key
		if label == "" {
			label = a.Name
		}
		onDemand := a.Processes != nil && a.Processes.Min == 0
		if onDemand {
			demand = append(demand, label)
		} else {
			kept++
		}
		up := (a.State == "running" || onDemand && a.State == "idle") && a.Processes != nil && a.Processes.Running >= a.Processes.Min
		if up {
			healthy++
			if !onDemand {
				running++
			}
		} else {
			bad = append(bad, label+" ("+a.State+")")
		}
	}
	word := "ok"
	if len(bad) > 0 {
		word = "degraded"
		if healthy == 0 {
			word = "failed"
		}
	}
	line := fmt.Sprintf("%s: %d/%d running", word, running, kept)
	if len(demand) > 0 {
		line += fmt.Sprintf(", %d on demand (%s)", len(demand), strings.Join(demand, ", "))
	}
	if len(bad) > 0 {
		line += "; " + strings.Join(bad, ", ")
	}
	return line
}
