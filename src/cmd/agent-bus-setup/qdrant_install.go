package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/parf/ai-agent-bus/internal/qdrant"
)

func qdrantVerb(verb, person string, dry bool) error {
	if !dry && os.Geteuid() != 0 {
		return fmt.Errorf("Qdrant host operations need root; run the owner-approved setup command with sudo")
	}
	var dir string
	uid, gid := 0, 0
	if verb == "install" {
		p, err := runnerFor(person, "")
		if err != nil {
			return err
		}
		account, err := user.Lookup(p.account)
		if err != nil {
			return fmt.Errorf("existing runner account %s is required", p.account)
		}
		uid, err = strconv.Atoi(account.Uid)
		if err != nil {
			return err
		}
		gid, err = strconv.Atoi(account.Gid)
		if err != nil {
			return err
		}
		dir = p.private
	}
	return qdrant.New(dir, uid, gid, os.Stdout).Do(verb, dry)
}
