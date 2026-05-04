// Package diskstats reports total and available bytes on the filesystem
// containing a given path.
package diskstats

// Stats returns (totalBytes, availBytes) for the filesystem containing path.
// Implementations live in diskstats_unix.go and diskstats_windows.go.
func Stats(path string) (int64, int64) {
	return getDiskStats(path)
}
