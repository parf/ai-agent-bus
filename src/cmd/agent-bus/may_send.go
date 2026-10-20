package main

import (
	"fmt"
	"net/url"
)

func maySend(args []string) error {
	pos, flags, parseErr := split(args)
	if err := only(flags, parseErr); err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("may-send wants one target name")
	}
	return get("/may-send", url.Values{"name": {pos[0]}})
}
