//go:build linux

package hwinfo

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func platformProbe(info *SystemInfo, path string) {
	info.CPUModel = readCPUModel()
	info.RAMBytes = readMemTotal()
	info.DiskModel = linuxDiskModel(path)
}

func readCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(data)) {
		// "model name : <value>"  or on ARM: "Hardware : <value>"
		key, val, ok := strings.Cut(strings.TrimRight(line, "\n"), ":")
		if !ok {
			continue
		}
		k := strings.TrimSpace(key)
		if k == "model name" || k == "Hardware" || k == "Model" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func readMemTotal() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for line := range strings.Lines(string(data)) {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}

// linuxDiskModel: df --output=source <path> → /dev/sdaN or /dev/nvme0n1pN →
// strip partition suffix → /sys/block/<dev>/device/model.
func linuxDiskModel(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	out := runCmd("df", "--output=source", abs)
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return ""
	}
	device := strings.TrimSpace(lines[1])
	device = strings.TrimPrefix(device, "/dev/")
	if device == "" {
		return ""
	}
	base := stripPartitionSuffix(device)
	model, err := os.ReadFile(filepath.Join("/sys/block", base, "device/model"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(model))
}

// stripPartitionSuffix removes a trailing partition number so /dev/sda1 → sda
// and /dev/nvme0n1p3 → nvme0n1.
func stripPartitionSuffix(dev string) string {
	if strings.HasPrefix(dev, "nvme") || strings.HasPrefix(dev, "mmcblk") {
		// nvme0n1p3, mmcblk0p1 → drop "p<n>" suffix
		if i := strings.LastIndex(dev, "p"); i > 0 {
			suffix := dev[i+1:]
			if _, err := strconv.Atoi(suffix); err == nil {
				return dev[:i]
			}
		}
		return dev
	}
	// sda1, vdb2, hdc10 → drop trailing digits
	end := len(dev)
	for end > 0 && dev[end-1] >= '0' && dev[end-1] <= '9' {
		end--
	}
	if end == 0 {
		return dev
	}
	return dev[:end]
}
