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

const Copyright = "© Sergey Porfiriev"

// Text preserves release and build on the first two lines for release readers.
func Text() string { return fmt.Sprintf("%s\nbuild_info: %s\n%s\n", String, Build, Copyright) }

// Build is stamped by build.sh through -ldflags, never guessed at runtime.
var Build = "development (unstamped)"

// Print handles the version query before any credentials, files or listeners.
func Print() bool {
	if len(os.Args) != 2 || (os.Args[1] != "--version" && os.Args[1] != "-version") {
		return false
	}
	fmt.Print(Text())
	return true
}
