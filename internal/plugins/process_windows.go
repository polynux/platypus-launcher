//go:build windows

package plugins

// Windows plugin supervision (issue #85): each plugin runs in its own process
// group plus a KILL_ON_JOB_CLOSE job object, so helpers it spawns (legendary,
// …) die with it — and so does the plugin itself if Platypus dies without a
// clean shutdown. That is the opposite of game jobs (internal/session keeps
// its jobs kill-on-close-free so games survive a core crash): plugins are
// part of the launcher and must never outlive it. The graceful stop stays the
// protocol shutdown (stop() closes stdin first); the job close below is the
// force path.

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procOpenProc          = kernel32.NewProc("OpenProcess")
	procCreateJobObjectW  = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob = kernel32.NewProc("SetInformationJobObject")
	procAssignToJob       = kernel32.NewProc("AssignProcessToJobObject")
	procCloseHandle       = kernel32.NewProc("CloseHandle")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobLimitKillOnJobClose            = 0x2000

	createNewProcessGroup = 0x200

	processSetQuota   = 0x0100
	processKillAccess = 0x0001 | processSetQuota
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
	priorityClass           uint32
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

// configurePluginProcess starts the plugin in its own process group. Go's
// exec already restricts inheritance to the stdio handles start() sets up
// (PROC_THREAD_ATTRIBUTE_HANDLE_LIST), so unrelated launcher handles stay
// out without NoInheritHandles (which would drop the protocol pipes too).
func configurePluginProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNewProcessGroup
}

// afterPluginStart puts a freshly started plugin in its own job object. A
// helper the plugin spawns between CreateProcess and this assignment escapes
// the job; that helper is reaped when the plugin's own teardown runs, and a
// plugin that never reaches assignment is killed directly below. Best
// effort: any failure leaves the plugin running without a job.
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
	procHandle, _, _ := procOpenProc.Call(uintptr(processKillAccess), 0, uintptr(pid))
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

// killPluginProcess ends the plugin and anything it spawned. Closing its job
// object terminates every member (KILL_ON_JOB_CLOSE), so no pid-based
// terminate follows — pids can be reused once Wait has reaped the leader.
// Without a job the leader is killed through Go's own handle, which reports
// ErrProcessDone instead of touching a stranger's pid when already reaped.
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
		return
	}
	_ = cmd.Process.Kill()
}
