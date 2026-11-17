//go:build windows

package session

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW     = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObj = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob   = kernel32.NewProc("AssignProcessToJobObject")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procTerminateProcess     = kernel32.NewProc("TerminateProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procSendCtrlEvent        = kernel32.NewProc("GenerateConsoleCtrlEvent")
	procCreateSnapshot       = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW      = kernel32.NewProc("Process32FirstW")
	procProcess32NextW       = kernel32.NewProc("Process32NextW")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobLimitKillOnJobClose            = 0x2000

	createNewProcessGroup = 0x200
	ctrlBreakEvent        = 1

	processTerminate = 0x0001
	processSetQuota  = 0x0100

	snapshotProcesses = 0x00000002
)

// jobBasicLimits mirrors JOBOBJECT_BASIC_LIMIT_INFORMATION; only LimitFlags
// is set (KILL_ON_JOB_CLOSE), the rest stays zero.
type jobBasicLimits struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessCount      uint32
	affinity                uintptr
	schedulingClass         uint32
}

type jobExtendedLimits struct {
	basic                 jobBasicLimits
	ioInfo                [6]uint64
	processMemoryLimit    uintptr
	jobMemoryLimit        uintptr
	peakProcessMemoryUsed uintptr
	peakJobMemoryUsed     uintptr
}

type processEntry32 struct {
	size            uint32
	usage           uint32
	processID       uint32
	defaultHeapID   uintptr
	moduleID        uint32
	threads         uint32
	parentProcessID uint32
	priClassBase    int32
	flags           uint32
	exeFile         [260]uint16
}

// gameJobs holds one KILL_ON_JOB_CLOSE job per directly launched game, keyed
// by the leader pid. Closing the job handle terminates the leader and every
// descendant that joined the job, without touching Platypus itself.
var gameJobs = struct {
	sync.Mutex
	m map[int]syscall.Handle
}{m: map[int]syscall.Handle{}}

// prepareProcess starts the game in its own process group, so a CTRL_BREAK
// stop reaches everything it spawned and never Platypus. NoInheritHandles
// keeps unrelated launcher handles out of the game; Go still inherits the
// stdio handles the tracker set up explicitly (log files, pipes).
func prepareProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = createNewProcessGroup
	cmd.SysProcAttr.NoInheritHandles = true
}

// assignGameJob puts a freshly started direct launch in its own job object.
// Callers hold no locks; every failure degrades to no job (a stop then only
// terminates the leader), never to a failed launch.
func assignGameJob(pid int) {
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return
	}
	limits := jobExtendedLimits{}
	limits.basic.limitFlags = jobLimitKillOnJobClose
	ok, _, _ := procSetInformationJobObj.Call(job,
		uintptr(jobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&limits)), uintptr(unsafe.Sizeof(limits)))
	if ok == 0 {
		procCloseHandle.Call(job)
		return
	}
	procHandle, _, _ := procOpenProcess.Call(
		uintptr(processTerminate|processSetQuota), 0, uintptr(pid))
	if procHandle == 0 {
		procCloseHandle.Call(job)
		return
	}
	ok, _, _ = procAssignProcessToJob.Call(job, procHandle)
	procCloseHandle.Call(procHandle)
	if ok == 0 {
		procCloseHandle.Call(job)
		return
	}
	gameJobs.Lock()
	gameJobs.m[pid] = syscall.Handle(job)
	gameJobs.Unlock()
}

// releaseGameJob drops the job handle after the leader exited on its own.
// The game is gone; if a descendant survived, the next stop finds no job and
// terminates it directly.
func releaseGameJob(pid int) {
	gameJobs.Lock()
	job, found := gameJobs.m[pid]
	if found {
		delete(gameJobs.m, pid)
	}
	gameJobs.Unlock()
	if found {
		procCloseHandle.Call(uintptr(job))
	}
}

// killProcesses signals the session. A graceful stop sends CTRL_BREAK to the
// game's own process group (best effort: without a console there is nothing
// to deliver it to, and the escalated force stop below still cleans up).
// A forced stop of a direct launch closes its job object, which terminates
// the leader and every descendant in the job; anything outside a job (client
// games, or a launch whose job assignment failed) is terminated through the
// parent links instead.
func killProcesses(pid int, direct bool, _ string, force bool) {
	if !force {
		procSendCtrlEvent.Call(uintptr(ctrlBreakEvent), uintptr(pid))
		return
	}
	gameJobs.Lock()
	job, found := gameJobs.m[pid]
	if found {
		delete(gameJobs.m, pid)
	}
	gameJobs.Unlock()
	if found {
		procCloseHandle.Call(uintptr(job))
	}
	if !direct || !found {
		for _, p := range descendantPids(pid) {
			terminatePid(p)
		}
	}
	terminatePid(pid)
}

func terminatePid(pid int) {
	h, _, _ := procOpenProcess.Call(uintptr(processTerminate), 0, uintptr(pid))
	if h == 0 {
		return
	}
	procTerminateProcess.Call(h, 1)
	procCloseHandle.Call(h)
}

// descendantPids returns every process below root, found through Toolhelp32
// parent links.
func descendantPids(root int) []int {
	snap, _, _ := procCreateSnapshot.Call(uintptr(snapshotProcesses), 0)
	if snap == uintptr(^uintptr(0)) {
		return nil
	}
	defer procCloseHandle.Call(snap)
	children := map[int][]int{}
	var entry processEntry32
	entry.size = uint32(unsafe.Sizeof(entry))
	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		children[int(entry.parentProcessID)] = append(children[int(entry.parentProcessID)], int(entry.processID))
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
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
