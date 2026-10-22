//go:build !windows

package session

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func prepareProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcesses(pid int, direct bool, token string, force bool) {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if direct {
		killGroup(pid, sig)
	} else {
		for _, p := range descendants(pid) {
			_ = syscall.Kill(p, sig)
		}
		_ = syscall.Kill(pid, sig)
	}
	if force && token != "" {
		for _, p := range pidsWithToken(token) {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}
}

func killGroup(pid int, sig syscall.Signal) {
	if pg, err := syscall.Getpgid(pid); err == nil && pg == pid && pg != syscall.Getpgrp() {
		_ = syscall.Kill(-pid, sig)
		return
	}
	_ = syscall.Kill(pid, sig)
}
func parentOf(pid int) int {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0
	}
	f := bytes.Fields(b[i+1:])
	if len(f) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(string(f[1]))
	return ppid
}
func descendants(root int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	children := map[int][]int{}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err == nil {
			if pp := parentOf(pid); pp > 0 {
				children[pp] = append(children[pp], pid)
			}
		}
	}
	var out, queue []int
	queue = append(queue, root)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, c := range children[p] {
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}
func pidsWithToken(token string) []int {
	needle := []byte(tokenEnv + "=" + token + "\x00")
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		if err == nil && bytes.Contains(env, needle) {
			out = append(out, pid)
		}
	}
	return out
}
