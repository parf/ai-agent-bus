//go:build cgo

package proctitle

import "github.com/erikdubbelboer/gspt"

func setTitle(title string) { gspt.SetProcTitle(title) }
