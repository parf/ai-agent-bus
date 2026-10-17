package sandbox

import (
	"bytes"
	"encoding/binary"
	"fmt"

	seccomp "github.com/elastic/go-seccomp-bpf"
	"golang.org/x/net/bpf"
)

// denied are the system calls an agent's code never needs and a way out of a
// sandbox often does: kernel modules and rebooting, mounting and namespaces,
// another process's memory, keys, clocks, BPF and perf, swap and quotas. A
// deny list in the spirit of the complement of systemd's @system-service
// (Plans/R1.0-Release/runner-implementation.md#sandboxing, Q166). Each is
// answered EPERM, so a program that probes for one carries on without it.
var denied = []string{
	"acct", "add_key", "adjtimex", "bpf", "clock_adjtime", "clock_settime",
	"delete_module", "finit_module", "fsconfig", "fsmount", "fsopen", "fspick",
	"init_module", "ioperm", "iopl", "kexec_file_load", "kexec_load", "keyctl",
	"lookup_dcookie", "mount", "mount_setattr", "move_mount", "name_to_handle_at",
	"open_by_handle_at", "open_tree", "perf_event_open", "pivot_root",
	"process_vm_readv", "process_vm_writev", "ptrace", "quotactl", "reboot",
	"request_key", "setns", "settimeofday", "swapoff", "swapon", "syslog",
	"umount2", "unshare", "userfaultfd", "uselib", "vhangup",
}

// Filter is the system-call filter compiled for this machine, as bubblewrap
// reads it from --seccomp: the program's struct sock_filter entries, in the
// machine's byte order. A system call of another architecture, x32 included,
// is refused as a whole.
func Filter() ([]byte, error) {
	policy := seccomp.Policy{
		DefaultAction: seccomp.ActionAllow,
		Syscalls:      []seccomp.SyscallGroup{{Action: seccomp.ActionErrno, Names: denied}},
	}
	insns, err := policy.Assemble()
	if err != nil {
		return nil, fmt.Errorf("the system-call filter: %w", err)
	}
	raw, err := bpf.Assemble(insns)
	if err != nil {
		return nil, fmt.Errorf("the system-call filter: %w", err)
	}
	var out bytes.Buffer
	for _, r := range raw {
		binary.Write(&out, binary.NativeEndian, struct {
			Op     uint16
			Jt, Jf uint8
			K      uint32
		}{r.Op, r.Jt, r.Jf, r.K})
	}
	return out.Bytes(), nil
}
