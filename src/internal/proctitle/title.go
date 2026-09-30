// Package proctitle updates ps without formatting anything on the request path.
package proctitle

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Start owns the process title until its returned stop function is called.
// A nil counter gives a static title. See docs/11-processes.md#process-titles.
func Start(name, detail string, calls *atomic.Uint64) func() {
	return StartNoted(name, detail, calls, nil)
}

// StartNoted is Start with a note read on every refresh, such as a runner's
// "away" while the daemon cannot be reached; an empty note shows nothing.
func StartNoted(name, detail string, calls *atomic.Uint64, note func() string) func() {
	show := func() {
		title := name + " " + version.String
		if calls != nil {
			title += fmt.Sprintf(" ; Calls: %d", calls.Load())
		}
		if detail != "" {
			title += " ; " + detail
		}
		if note != nil {
			if n := note(); n != "" {
				title += " ; " + n
			}
		}
		gspt.SetProcTitle(title)
	}
	show()
	if calls == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				show()
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
