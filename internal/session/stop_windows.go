//go:build windows

package session

// Windows process supervision (issue #85): a directly launched game runs in
// its own process group plus a job object. A graceful stop is CTRL_BREAK to
// that group (best effort); a forced stop terminates the job object, which
// ends the leader and every descendant that joined it, without touching
// Platypus itself. This is not a real graceful stop for GUI games: those
// need WM_CLOSE delivered to their windows, which is a follow-up issue and
// is not implemented here.
//
// Game jobs deliberately lack JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, like Unix
// process groups: games survive a core crash, and the job handle lives until
// the game's session ends (StopAll or the tracker's release after Wait), so
// a stub launcher exiting early never kills the real game. Plugin jobs keep
// KILL_ON_JOB_CLOSE instead: plugins must die with the core.

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
	procTerminateJobObject   = kernel32.NewProc("TerminateJobObject")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procTerminateProcess     = kernel32.NewProc("TerminateProcess")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procSendCtrlEvent        = kernel32.NewProc("GenerateConsoleCtrlEvent")
	procCreateSnapshot       = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW      = kernel32.NewProc("Process32FirstW")
	procProcess32NextW       = kernel32.NewProc("Process32NextW")
	procGetProcessTimes      = kernel32.NewProc("GetProcessTimes")
)

const (
	jobObjectExtendedLimitInformation = 9

	createNewProcessGroup = 0x200
	ctrlBreakEvent        = 1

	processTerminate    = 0x0001
	processSetQuota     = 0x0100
	processQueryLimited = 0x1000

	snapshotProcesses = 0x00000002
)

// jobBasicLimits mirrors JOBOBJECT_BASIC_LIMIT_INFORMATION; everything stays
// zero (no constraints, and deliberately no KILL_ON_JOB_CLOSE: games must
// survive a core crash, like on Unix).
type jobBasicLimits struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessCount      uint32
	affinity                uintptr
	priorityClass           uint32
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

// gameJobs holds one job per directly launched game, keyed by the leader
// pid, until its session ends. Terminating the job ends the tree; closing
// the handle alone ends nothing (no KILL_ON_JOB_CLOSE by design).
var gameJobs = struct {
	sync.Mutex
	m map[int]syscall.Handle
}{m: map[int]syscall.Handle{}}

// prepareProcess starts the game in its own process group, so a CTRL_BREAK
// stop reaches everything it spawned and never Platypus. Go's exec already
// restricts inheritance to the stdio handles set up for the child
// (PROC_THREAD_ATTRIBUTE_HANDLE_LIST), so unrelated launcher handles stay
// out without NoInheritHandles (which would drop the stdio pipes too).
func prepareProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNewProcessGroup
}

// assignGameJob puts a freshly started direct launch in its own job object.
// A child the game spawns between CreateProcess and this assignment escapes
// the job; the force stop below then only terminates job members, so such an
// early child is reaped by the client's own teardown or the next launch's
// stop, never Platypus. Every failure degrades to no job (a stop then
// terminates through the parent links), never to a failed launch.
func assignGameJob(pid int) {
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return
	}
	limits := jobExtendedLimits{}
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

// releaseGameJob drops the job handle when the game's session ends. The game
// is gone; a surviving descendant keeps running (no KILL_ON_JOB_CLOSE), and
// the next stop for that pid finds no job and terminates it directly.
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

// killProcesses signals the session. A graceful stop sends CTRL_BREAK, but
// only to a directly launched game in its own process group — never to a
// client-launched pid, whose group belongs to the store client. (Without a
// console there is nothing to deliver it to; the escalated force stop still
// cleans up. Real GUI grace needs WM_CLOSE, still a follow-up.)
// A forced stop of a direct launch terminates its job object, which ends the
// leader and every descendant in the job; terminating/closing the job is
// enough, so no pid-based terminate follows. Anything outside a job (client
// games, or a launch whose job assignment failed) is terminated through the
// parent links instead.
func killProcesses(pid int, direct bool, _ string, force bool) {
	if !force {
		if direct {
			procSendCtrlEvent.Call(uintptr(ctrlBreakEvent), uintptr(pid))
		}
		return
	}
	gameJobs.Lock()
	job, found := gameJobs.m[pid]
	if found {
		delete(gameJobs.m, pid)
	}
	gameJobs.Unlock()
	if found {
		procTerminateJobObject.Call(uintptr(job), 1)
		procCloseHandle.Call(uintptr(job))
		return
	}
	for _, p := range descendantPids(pid) {
		terminatePid(p)
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
// parent links. A link is kept only when the child was created no earlier
// than its parent: a reused pid from an older, unrelated process must never
// be swept into another game's tree. When either creation time is
// unavailable the link is kept (best effort).
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
	times := map[int]uint64{}
	creation := func(pid int) (uint64, bool) {
		if t, found := times[pid]; found {
			return t, true
		}
		t, found := creationTimeOf(pid)
		if found {
			times[pid] = t
		}
		return t, found
	}
	var out, queue []int
	queue = append(queue, root)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		pTime, pOK := creation(p)
		for _, c := range children[p] {
			if pOK {
				if cTime, cOK := creation(c); cOK && cTime < pTime {
					continue
				}
			}
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}

// creationTimeOf returns the process creation time in 100ns ticks since
// 1601, or false when the process can't be opened or queried.
func creationTimeOf(pid int) (uint64, bool) {
	h, _, _ := procOpenProcess.Call(uintptr(processQueryLimited), 0, uintptr(pid))
	if h == 0 {
		return 0, false
	}
	defer procCloseHandle.Call(h)
	var creation, exited, kernel, user uint64
	ok, _, _ := procGetProcessTimes.Call(h,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return 0, false
	}
	return creation, true
}
