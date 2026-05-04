//go:build darwin

package hwinfo

import (
	"path/filepath"
	"strconv"
	"strings"
)

func platformProbe(info *SystemInfo, path string) {
	info.CPUModel = runCmd("sysctl", "-n", "machdep.cpu.brand_string")
	if mem := runCmd("sysctl", "-n", "hw.memsize"); mem != "" {
		if v, err := strconv.ParseInt(mem, 10, 64); err == nil {
			info.RAMBytes = v
		}
	}
	info.DiskModel = darwinDiskModel(path)
}

// darwinDiskModel: df -P <path> → device → diskutil info -plist <device> →
// scan plist text for the "Media Name" key. We avoid pulling in encoding/plist
// (not stdlib) by string-scanning the output.
func darwinDiskModel(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	df := runCmd("df", "-P", abs)
	if df == "" {
		return ""
	}
	// df output: header line, then "<device> <blocks> <used> <avail> <cap> <mount>"
	lines := strings.Split(df, "\n")
	if len(lines) < 2 {
		return ""
	}
	fields := strings.Fields(lines[1])
	if len(fields) == 0 {
		return ""
	}
	device := fields[0]

	plist := runCmd("diskutil", "info", "-plist", device)
	if plist == "" {
		return ""
	}
	// Extract the value following the <key>Media Name</key> entry.
	if v := plistString(plist, "Media Name"); v != "" {
		return v
	}
	if v := plistString(plist, "IORegistryEntryName"); v != "" {
		return v
	}
	return ""
}

func plistString(plist, key string) string {
	marker := "<key>" + key + "</key>"
	idx := strings.Index(plist, marker)
	if idx < 0 {
		return ""
	}
	rest := plist[idx+len(marker):]
	openTag := strings.Index(rest, "<string>")
	if openTag < 0 {
		return ""
	}
	rest = rest[openTag+len("<string>"):]
	closeTag := strings.Index(rest, "</string>")
	if closeTag < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:closeTag])
}
