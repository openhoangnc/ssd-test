package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {

	// check disk free space
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(".", &fs)
	if err != nil {
		panic(err)
	}

	diskSize := fs.Blocks * uint64(fs.Bsize)
	fmt.Printf("Disk size: %s\n", formatBytes(int64(diskSize)))

	freeSpace := fs.Bavail * uint64(fs.Bsize)
	fmt.Printf("Free space: %s\n", formatBytes(int64(freeSpace)))

	fileSize := int64(freeSpace) - int64(diskSize/100) // leave 1% free
	if fileSize < 0 {
		fmt.Printf("Not enough free space, need at least %s\n", formatBytes(int64(diskSize)/100))
		return
	}

	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	filename := fmt.Sprintf("disk-test-%x.tmp", randBytes)

	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	cleanup := func() {
		fmt.Println("\nCleaning up...")
		f.Close()
		os.Remove(filename)
		fmt.Println("Removed file", filename)
	}

	fmt.Printf("Writing %s to %s\n", formatBytes(fileSize), filename)
	if err := f.Truncate(fileSize); err != nil {
		fmt.Printf("Error allocating file %s with size %s: %s\n", filename, formatBytes(fileSize), err)
		cleanup()
		return
	}

	// handle signals
	sign := make(chan os.Signal, 1)
	signal.Notify(sign, os.Interrupt, os.Kill)
	stopped := false
	go func() {
		<-sign
		stopped = true
	}()

	randData := make([]byte, 1*1024*1024)
	rand.Read(randData)

	totalWritten := int64(0)
	blockWritten := int64(0)
	writeStart := time.Now()
	blockStart := time.Now()
	maxSpeed := 0.0

	for {
		if stopped || totalWritten >= fileSize {
			break
		}

		written, err := f.Write(randData)

		if err != nil {
			if !strings.Contains(err.Error(), "no space left on device") {
				fmt.Println("\nError writing to file:", err)
			}
			break
		}

		totalWritten += int64(written)
		blockWritten += int64(written)

		if time.Since(blockStart) > 1*time.Second {
			if err := f.Sync(); err != nil {
				fmt.Println("\nError syncing file:", err)
				break
			}

			totalElapsed := time.Since(writeStart)
			blockElapsed := time.Since(blockStart)

			blockSpeed := float64(blockWritten) / blockElapsed.Seconds()
			if blockSpeed > maxSpeed {
				maxSpeed = blockSpeed
			}
			avgSpeed := float64(totalWritten) / totalElapsed.Seconds()
			fmt.Printf("\rWrote %s in %s (avg %s/s) (current %s/s) (max %s/s)      ",
				formatBytes(totalWritten), formatDuration(totalElapsed),
				formatBytes(int64(avgSpeed)),
				formatBytes(int64(blockSpeed)),
				formatBytes(int64(maxSpeed)),
			)
			blockWritten = 0
			blockStart = time.Now()
		}
	}

	cleanup()
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2fKB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2fMB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2fGB", float64(bytes)/(1024*1024*1024))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d/time.Millisecond)
	} else if d < time.Minute {
		return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
	} else {
		return fmt.Sprintf("%.2fm", float64(d)/float64(time.Minute))
	}
}
