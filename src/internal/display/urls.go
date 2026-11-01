package display

import (
	"github.com/parf/ai-agent-bus/internal/version"
	"strings"
)

// Homepage is the public project site; Source owns code and documentation.
const Homepage = "https://parf.dev/agent-bus/"
const Source = "https://github.com/parf/ai-agent-bus"
const SourceURL = Source + "/blob/main"

// Project metadata is shared by daemon descriptions and faces.
const License = "PolyForm-Noncommercial-1.0.0"
const Author = "Sergey Porfiriev"

// BusDescription is #bus's help description; docs/agents/README.md's agent list is generated from it.
const BusDescription = "The daemon on the bus; read-only methods use your authority"

// BusDocs is #bus's page in the bundled agents guide.
const BusDocs = SourceURL + "/docs/agents/bus.md"

// Footer is the common copyright line on command help.
func Footer() string {
	return "v" + version.String + " · PolyForm-NC · " + strings.TrimPrefix(Homepage, "https://") + " · " + version.Copyright
}
