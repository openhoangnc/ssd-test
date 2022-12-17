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

	// leave 1% free space, max 1GB
	leaveSpace := diskSize / 100
	if leaveSpace > 1024*1024*1024 {
		leaveSpace = 1024 * 1024 * 1024
	}

	fileSize := int64(freeSpace) - int64(leaveSpace)
	if fileSize < 0 {
		fmt.Printf("Not enough free space, need at least %s\n", formatBytes(int64(leaveSpace)))
		return
	}

	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	filename := fmt.Sprintf("ssd-test-%x.tmp", randBytes)

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
	signal.Notify(sign, os.Interrupt, syscall.SIGTERM)
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

	firstWrite := true
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

			s := "" +
				"Written size  : " + padStr(formatBytes(totalWritten)) + "\n" +
				"Elapsed time  : " + padStr(formatDuration(totalElapsed)) + "\n" +
				"Average speed : " + padStr(formatBytes(int64(avgSpeed))) + "/s\n" +
				"Current speed : " + padStr(formatBytes(int64(blockSpeed))) + "/s\n" +
				"Max speed     : " + padStr(formatBytes(int64(maxSpeed))) + "/s\n"

			if firstWrite {
				firstWrite = false
			} else {
				fmt.Printf("\033[%dA", strings.Count(s, "\n"))
			}
			fmt.Print(s)

			blockWritten = 0
			blockStart = time.Now()
		}
	}

	cleanup()
}

func padStr(s string) string {
	return fmt.Sprintf("%10s", s)
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d  B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KiB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MiB", float64(bytes)/(1024*1024))
	} else if bytes < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.2f GiB", float64(bytes)/(1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2f TiB", float64(bytes)/(1024*1024*1024*1024))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms ", d/time.Millisecond)
	} else if d < time.Minute {
		return fmt.Sprintf("%.2f s  ", float64(d)/float64(time.Second))
	} else {
		return fmt.Sprintf("%.2f m  ", float64(d)/float64(time.Minute))
	}
}
