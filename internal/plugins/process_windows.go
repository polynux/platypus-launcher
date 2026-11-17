//go:build windows

package plugins

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procCreateJobObjectW  = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob = kernel32.NewProc("SetInformationJobObject")
	procAssignToJob       = kernel32.NewProc("AssignProcessToJobObject")
	procOpenProcess       = kernel32.NewProc("OpenProcess")
	procTerminateProcess  = kernel32.NewProc("TerminateProcess")
	procCloseHandle       = kernel32.NewProc("CloseHandle")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobLimitKillOnJobClose            = 0x2000

	createNewProcessGroup = 0x200

	processTerminate = 0x0001
	processSetQuota  = 0x0100
)

// jobLimits mirrors JOBOBJECT_EXTENDED_LIMIT_INFORMATION with only the
// KILL_ON_JOB_CLOSE flag set; every member dies when the last job handle
// closes, which is also what kills a plugin that outlives Platypus.
type jobLimits struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessCount      uint32
	affinity                uintptr
	schedulingClass         uint32
	ioInfo                  [6]uint64
	processMemoryLimit      uintptr
	jobMemoryLimit          uintptr
	peakProcessMemoryUsed   uintptr
	peakJobMemoryUsed       uintptr
}

// pluginJobs holds one KILL_ON_JOB_CLOSE job per running plugin, keyed by its
// leader pid.
var pluginJobs = struct {
	sync.Mutex
	m map[int]syscall.Handle
}{m: map[int]syscall.Handle{}}

// configurePluginProcess starts the plugin in its own process group and
// keeps unrelated launcher handles out of it. Go still inherits the stdio
// handles start() set up explicitly (the protocol pipes, the log file), so
// the protocol stream is unaffected.
func configurePluginProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = createNewProcessGroup
	cmd.SysProcAttr.NoInheritHandles = true
}

// afterPluginStart puts a freshly started plugin in its own job object, so
// helpers it spawns (legendary, …) die with it, and so does the plugin if
// Platypus dies without a clean shutdown. Best effort: any failure leaves
// the plugin running without a job.
func afterPluginStart(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return
	}
	limits := jobLimits{limitFlags: jobLimitKillOnJobClose}
	ok, _, _ := procSetInformationJob.Call(job,
		uintptr(jobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&limits)), uintptr(unsafe.Sizeof(limits)))
	if ok == 0 {
		procCloseHandle.Call(job)
		return
	}
	pid := cmd.Process.Pid
	procHandle, _, _ := procOpenProcess.Call(
		uintptr(processTerminate|processSetQuota), 0, uintptr(pid))
	if procHandle == 0 {
		procCloseHandle.Call(job)
		return
	}
	ok, _, _ = procAssignToJob.Call(job, procHandle)
	procCloseHandle.Call(procHandle)
	if ok == 0 {
		procCloseHandle.Call(job)
		return
	}
	pluginJobs.Lock()
	pluginJobs.m[pid] = syscall.Handle(job)
	pluginJobs.Unlock()
}

// killPluginProcess ends the plugin and anything it spawned: closing its job
// object terminates every member, and the leader is terminated directly in
// case it never joined a job. The waiter's recordCrash already ran; the job
// handle is closed exactly once here.
func killPluginProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	pluginJobs.Lock()
	job, found := pluginJobs.m[pid]
	if found {
		delete(pluginJobs.m, pid)
	}
	pluginJobs.Unlock()
	if found {
		procCloseHandle.Call(uintptr(job))
	}
	h, _, _ := procOpenProcess.Call(uintptr(processTerminate), 0, uintptr(pid))
	if h == 0 {
		return
	}
	procTerminateProcess.Call(h, 1)
	procCloseHandle.Call(h)
}
