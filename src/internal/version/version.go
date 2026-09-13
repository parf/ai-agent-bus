// Package version shares one release number across every program and face.
package version

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed VERSION
var raw string

var String = strings.TrimSpace(raw)

// Build is stamped by build.sh through -ldflags, never guessed at runtime.
var Build = "development (unstamped)"

// Print handles the version query before any credentials, files or listeners.
func Print() bool {
	if len(os.Args) != 2 || (os.Args[1] != "--version" && os.Args[1] != "-version") {
		return false
	}
	fmt.Printf("%s\nbuild_info: %s\n", String, Build)
	return true
}
