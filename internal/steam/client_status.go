package steam

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"platypus/internal/types"
)

var actionRE = regexp.MustCompile(`GameAction \[AppID ([0-9]+), ActionID ([0-9]+)\] : LaunchApp changed task to ([A-Za-z]+)`)
var processAddedRE = regexp.MustCompile(`Game process added : AppID ([0-9]+)\b`)
var logTimeRE = regexp.MustCompile(`^\[([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2})\]`)

// ClientStatus reads only the tail of Steam's console log and the app manifest.
// It never changes Steam state. The log tail is bounded to the latest 4 MiB.
func ClientStatus(appID string, since ...time.Time) *types.ClientStatus {
	root := Root()
	if root == "" || appID == "" {
		return nil
	}
	logPath := filepath.Join(root, "logs", "console_log.txt")
	f, err := os.Open(logPath)
	if err != nil {
		return nil
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil
	}
	const tail = 4 << 20
	offset := st.Size() - tail
	if offset < 0 {
		offset = 0
	}
	_, _ = f.Seek(offset, 0)
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	state := ""
	worked := false
	handedOff := false
	for s.Scan() {
		line := s.Text()
		if processMatch := processAddedRE.FindStringSubmatch(line); len(processMatch) == 2 && processMatch[1] == appID && afterLaunchTime(line, since) {
			state = "launching"
			handedOff = true
			continue
		}
		m := actionRE.FindStringSubmatch(line)
		if len(m) != 4 || m[1] != appID {
			continue
		}
		if !afterLaunchTime(line, since) {
			continue
		}
		switch m[3] {
		case "CheckShaderDepotManifest":
			state = "shaders"
			worked = true
		case "DownloadingDepots":
			state = "downloading"
			worked = true
		case "SiteLicenseSeatCheckout", "DelayLaunch", "CreatingProcess", "WaitingGameWindow":
			state = "launching"
			handedOff = true
		case "Completed":
			// Steam completed its launch task by handing off to the game. The
			// game's process can take several seconds to appear under Proton.
			state = "launching"
			handedOff = true
		}
	}
	_ = f.Close()
	status := &types.ClientStatus{State: state}
	if state == "" {
		status.State = "launching"
	}
	for _, lib := range LibraryFolders(root) {
		path := filepath.Join(lib, "steamapps", "appmanifest_"+appID+".acf")
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(b)
		down := manifestInt(text, "BytesDownloaded")
		total := manifestInt(text, "BytesToDownload")
		if total > 0 && down < total {
			status.State = "updating"
			status.Bytes = down
			status.Total = total
			p := float64(down) / float64(total)
			status.Progress = &p
		}
		flags := manifestInt(text, "StateFlags")
		if flags&131072 != 0 {
			status.State = "validating"
		} else if worked && !handedOff && (status.State == "downloading" || status.State == "updating" || status.State == "shaders") {
			if flags&4 != 0 && flags&2 == 0 {
				status.State = "idle"
			}
		}
		break
	}
	return status
}

func afterLaunchTime(line string, since []time.Time) bool {
	if len(since) == 0 || since[0].IsZero() {
		return true
	}
	tm := logTimeRE.FindStringSubmatch(line)
	if len(tm) != 2 {
		return false
	}
	at, err := time.ParseInLocation("2006-01-02 15:04:05", tm[1], time.Local)
	return err == nil && !at.Before(since[0].Add(-time.Second))
}

func manifestInt(text, key string) int64 {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s+"([0-9]+)"`)
	m := re.FindStringSubmatch(text)
	if len(m) != 2 {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(m[1]), 10, 64)
	return n
}
