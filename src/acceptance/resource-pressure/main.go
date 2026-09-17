// Disposable acceptance pressure only; never included in the installed package.
// The root driver puts a bounded, unprivileged helper in the web's actual cgroup.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "pressure" {
		switch os.Args[2] {
		case "memory":
			var live [][]byte
			for range 20 { // bounded at 320 MiB even if the limit is missing
				b := make([]byte, 16<<20)
				for i := 0; i < len(b); i += 4096 {
					b[i] = 1
				}
				live = append(live, b)
			}
			fmt.Println("allocated 320 MiB without group OOM")
			time.Sleep(time.Second)
			runtime.KeepAlive(live)
		case "cpu":
			runtime.GOMAXPROCS(4)
			var wg sync.WaitGroup
			for range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					until := time.Now().Add(4 * time.Second)
					for time.Now().Before(until) {
					}
				}()
			}
			wg.Wait()
		case "pids":
			var children []*exec.Cmd
			for range 80 { // finite even when pids.max is removed
				cmd := exec.Command("/usr/bin/sleep", "3")
				if err := cmd.Start(); err != nil {
					fmt.Println("fork refused:", err)
					break
				}
				children = append(children, cmd)
			}
			fmt.Println("started children", len(children))
			for _, cmd := range children {
				_ = cmd.Wait()
			}
		default:
			panic("unknown pressure")
		}
		return
	}
	if len(os.Args) != 5 {
		panic("usage: resource-pressure CGROUP UID GID memory|cpu|pids")
	}
	uid, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}
	gid, err := strconv.Atoi(os.Args[3])
	if err != nil {
		panic(err)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "pressure", os.Args[4])
	cmd.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: int(f.Fd()), Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}, Pdeathsig: syscall.SIGKILL}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	membership, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", cmd.Process.Pid))
	if err != nil {
		panic(err)
	}
	want, err := filepath.Abs(os.Args[1])
	if err != nil {
		panic(err)
	}
	got := strings.TrimSpace(strings.TrimPrefix(string(membership), "0::"))
	want = strings.TrimPrefix(want, "/sys/fs/cgroup")
	if got != want {
		_ = cmd.Process.Kill()
		panic(fmt.Sprintf("pressure helper entered %q, want %q", got, want))
	}
	fmt.Println("pressure member", cmd.Process.Pid, strings.TrimSpace(string(membership)))
	if err := cmd.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
