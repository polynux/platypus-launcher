package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// What each drive is (issue #148): the filesystem type from mountinfo, the
// device kind from sysfs and the label udev recorded. Every test feeds fake
// mountinfo/sysfs content: no test reads the machine it runs on.

func TestDiskMountsParseFilesystemAndDevice(t *testing.T) {
	t.Parallel()
	data := "26 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw\n" +
		"27 26 8:17 / /mnt/games rw,relatime master:1 - btrfs /dev/nvme0n1p1 rw\n" +
		"28 27 8:33 / /mnt/label rw - ext4 /dev/disk/by-label/My\\040Games rw\n" +
		"29 27 0:44 / /mnt/net rw - nfs4 server:/export rw\n"
	mounts := parseDiskMounts([]byte(data))
	if len(mounts) != 4 {
		t.Fatalf("mounts = %+v, want 4", mounts)
	}
	want := []diskMount{
		{point: "/", fsType: "ext4", source: "/dev/sda1"},
		{point: "/mnt/games", fsType: "btrfs", source: "/dev/nvme0n1p1"},
		{point: "/mnt/label", fsType: "ext4", source: "/dev/disk/by-label/My Games"},
		{point: "/mnt/net", fsType: "nfs4", source: "server:/export"},
	}
	for i, m := range mounts {
		if m.point != want[i].point || m.fsType != want[i].fsType || m.source != want[i].source {
			t.Fatalf("mount %d = %+v, want %+v", i, m, want[i])
		}
	}
}

// fakeSysfs answers reads from a map instead of the real /sys.
func fakeSysfs(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if v, ok := files[path]; ok {
			return []byte(v), nil
		}
		return nil, errors.New("no such file")
	}
}

func TestDeviceKindFromTheDeviceAndItsFilesystem(t *testing.T) {
	t.Parallel()
	sysfs := fakeSysfs(map[string]string{
		"/sys/block/sda/queue/rotational":     "0\n",
		"/sys/block/sdb/queue/rotational":     "1\n",
		"/sys/block/sdc/removable":            "1\n",
		"/sys/block/nvme0n1/queue/rotational": "0\n",
	})
	cases := []struct {
		name  string
		mount diskMount
		want  string
	}{
		{"network filesystem type", diskMount{fsType: "cifs", source: "//host/share"}, "network"},
		{"network filesystem without a slash", diskMount{fsType: "nfs4", source: "server:/export"}, "network"},
		{"removable beats its disk's flag", diskMount{fsType: "vfat", source: "/dev/sdc1"}, "removable"},
		{"nvme partition reads its disk", diskMount{fsType: "ext4", source: "/dev/nvme0n1p2"}, "nvme"},
		{"ssd", diskMount{fsType: "ext4", source: "/dev/sda2"}, "ssd"},
		{"hdd", diskMount{fsType: "ext4", source: "/dev/sdb1"}, "hdd"},
		{"unknown device", diskMount{fsType: "ext4", source: "/dev/mapper/vg-root"}, ""},
		{"loop image", diskMount{fsType: "squashfs", source: "/dev/loop0"}, ""},
		{"no device at all", diskMount{fsType: "tmpfs", source: "tmpfs"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := deviceKind(c.mount, sysfs); got != c.want {
				t.Fatalf("deviceKind(%+v) = %q, want %q", c.mount, got, c.want)
			}
		})
	}
}

func TestFilesystemLabelFromUdev(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, target := range map[string]string{
		"JEUX":          "../../sdb1",
		`windows\x20xp`: "../../sdc1",
	} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]string{
		"/dev/sdb1": "JEUX",
		"/dev/sdc1": "windows xp", // udev's blank escape is undone
		"/dev/sdd1": "",
		"/dev/sdb":  "", // a label names a filesystem, not its disk
		"tmpfs":     "",
	}
	for source, want := range cases {
		if got := filesystemLabel(source, dir); got != want {
			t.Fatalf("filesystemLabel(%q) = %q, want %q", source, got, want)
		}
	}
	if got := filesystemLabel("/dev/sdb1", filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("label without a by-label dir = %q, want none", got)
	}
}

func TestDiskUsageDriveCarriesWhatItIs(t *testing.T) {
	t.Parallel()
	drive, home := evalTemp(t), evalTemp(t)
	dir := filepath.Join(drive, "Games", "One")
	writeFileOf(t, dir, 1<<20)
	game := types.Game{ID: "one", Title: "One", Source: "custom", Store: types.StoreCustom, InstallDir: dir, Executable: filepath.Join(dir, "run.sh"), IsInstalled: true, Native: true}
	var walks walkLog
	mounts := []diskMount{{point: drive, fsType: "btrfs", source: "/dev/nvme0n1p1", kind: "nvme", label: "JEUX"}}
	usage := buildDiskUsage([]types.Game{game}, nil, home, filepath.Join(home, "prefixes"), mounts,
		func(string) (uint64, uint64, error) { return 10 << 30, 5 << 30, nil }, walks.measure)
	got := driveByID(t, usage, drive)
	if got.FsType != "btrfs" || got.DeviceKind != "nvme" || got.Label != "JEUX" {
		t.Fatalf("drive = %+v, want its filesystem, device kind and label", got)
	}
}
