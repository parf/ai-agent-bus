package main

import (
	"os"
	"testing"

	"github.com/parf/ai-agent-bus/internal/authkeys"
)

// The tests run as agent-bus-admin.test, and `user add --admin` forces the key to the running program.
func TestMain(m *testing.M) {
	authkeys.Programs = append(authkeys.Programs, "agent-bus-admin.test")
	os.Exit(m.Run())
}
