package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
)

const supervisorPIDEnv = "AGENT_BUS_SUPERVISOR_PID"

// The spawning supervisor supplies its PID before exec. Capturing Getppid in
// the child would accept launchd or a subreaper if the supervisor died first.
// Kept platform-independent so the actual fallback can be checked on Linux.
func watchSupervisorParent() error {
	parent, err := strconv.Atoi(os.Getenv(supervisorPIDEnv))
	if err != nil || parent <= 1 {
		return fmt.Errorf("missing or invalid supervisor PID")
	}
	if os.Getppid() != parent {
		return fmt.Errorf("supervisor %d is already gone; refusing orphan startup", parent)
	}
	go func() {
		for os.Getppid() == parent {
			time.Sleep(time.Second)
		}
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	return nil
}
