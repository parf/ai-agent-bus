package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	runnerUnitPath = "/etc/systemd/system/agent-bus-runner.service"
	runnerUnitName = "agent-bus-runner.service"
)

// runnerUnit is the managed runner's unit
// (Plans/R1.0-Release/runner-implementation.md#runner-unit): its own account,
// only its home writable, a cgroup subtree delegated to it so it can give each
// agent a cgroup of its own, and no capability. Wanted beside a local bus and
// ordered after it, never Required: a bus that is away is not a runner that
// failed.
func runnerUnit(exe string) string {
	return fmt.Sprintf(`[Unit]
Description=agent-bus managed runner
Documentation=https://github.com/parf/ai-agent-bus/blob/main/Plans/R1.0-Release/runner.md
Wants=agent-busd.service
After=agent-busd.service

[Service]
User=%[1]s
ExecStart=%[2]s --home %[3]s
WorkingDirectory=%[4]s
Restart=on-failure
RestartSec=2
# The runner gives each agent a cgroup of its own inside this subtree
# (Plans/R1.0-Release/runner.md#sandboxing); no capability is needed for it.
Delegate=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%[4]s

[Install]
WantedBy=multi-user.target
`, runAccount, exe, stateRoot, runHome)
}

// installRunner writes, enables and (re)starts the runner's unit. A node
// without it still serves every other face, so the caller reports a failure
// rather than ending the install.
func installRunner(exeDir string) error {
	exe := filepath.Join(exeDir, "agent-bus-runner")
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("the release has no runner: %w", err)
	}
	if err := os.WriteFile(runnerUnitPath, []byte(runnerUnit(exe)), 0o644); err != nil {
		return err
	}
	for _, step := range [][]string{{"daemon-reload"}, {"enable", runnerUnitName}, {"restart", runnerUnitName}} {
		if err := run("systemctl", step...); err != nil {
			return err
		}
	}
	return nil
}
