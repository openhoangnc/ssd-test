//go:build windows

package hwinfo

import (
	"strconv"
	"strings"
)

func platformProbe(info *SystemInfo, path string) {
	info.CPUModel = runCmd("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Select-Object -First 1 -ExpandProperty Name)")

	memOut := runCmd("powershell", "-NoProfile", "-Command",
		"[int64](Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory")
	if v, err := strconv.ParseInt(strings.TrimSpace(memOut), 10, 64); err == nil {
		info.RAMBytes = v
	}

	info.DiskModel = windowsDiskModel(path)
}

// windowsDiskModel: from a path, find the volume's drive letter, look up the
// partition that backs it, then the disk's Model property.
func windowsDiskModel(path string) string {
	letter := strings.ToUpper(strings.TrimSpace(path))
	if len(letter) >= 2 && letter[1] == ':' {
		letter = string(letter[0]) + ":"
	} else {
		letter = "C:"
	}
	cmd := "$d=Get-Partition -DriveLetter " + string(letter[0]) +
		" | Get-Disk; $d.FriendlyName"
	out := runCmd("powershell", "-NoProfile", "-Command", cmd)
	return strings.TrimSpace(out)
}
