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
	diskSize, freeSpace := getDiskStats(".")

	fmt.Printf("Disk size: %s\n", formatBytes(int64(diskSize)))

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

	file, err := createTestFile(filename, fileSize)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Setup cleanup to be called on exit
	cleanup := func() {
		fmt.Println("\nCleaning up...")
		file.Close()
		os.Remove(filename)
		fmt.Println("Removed file", filename)
	}
	defer cleanup()

	// Setup signal handling for graceful termination
	stopChan, signalReceived := setupSignalHandler()

	// Prepare random data for writing
	randData := make([]byte, 1*1024*1024)
	rand.Read(randData)

	// Perform the write test
	performWriteTest(file, fileSize, randData, stopChan, signalReceived)
}

// createTestFile creates a file with the specified name and size
func createTestFile(filename string, size int64) (*os.File, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	fmt.Printf("Writing %s to %s\n", formatBytes(size), filename)
	if err := f.Truncate(size); err != nil {
		f.Close()
		os.Remove(filename)
		return nil, fmt.Errorf("error allocating file %s with size %s: %w",
			filename, formatBytes(size), err)
	}

	return f, nil
}

// setupSignalHandler creates a channel that is closed when SIGINT or SIGTERM is received
func setupSignalHandler() (<-chan struct{}, *bool) {
	stopChan := make(chan struct{})
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	signalReceived := false

	go func() {
		<-signalChan
		signalReceived = true
		close(stopChan)
	}()

	return stopChan, &signalReceived
}

// performWriteTest writes random data to the file and displays progress
func performWriteTest(f *os.File, fileSize int64, randData []byte, stopChan <-chan struct{}, signalReceived *bool) {
	totalWritten := int64(0)
	blockWritten := int64(0)
	writeStart := time.Now()
	blockStart := time.Now()
	maxSpeed := 0.0

	firstWrite := true
	for {
		select {
		case <-stopChan:
			// Just exit the loop without another progress update when signal is received
			return
		default:
			if totalWritten >= fileSize {
				return
			}

			written, err := f.Write(randData)

			if err != nil {
				if !strings.Contains(err.Error(), "no space left on device") {
					fmt.Println("\nError writing to file:", err)
				}
				return
			}

			totalWritten += int64(written)
			blockWritten += int64(written)

			// Only update progress if no signal has been received
			if time.Since(blockStart) > 1*time.Second && !*signalReceived {
				updateProgress(f, totalWritten, blockWritten, writeStart,
					blockStart, &maxSpeed, &firstWrite)
				blockWritten = 0
				blockStart = time.Now()
			}
		}
	}
}

// updateProgress displays the current test progress
func updateProgress(f *os.File, totalWritten, blockWritten int64,
	writeStart, blockStart time.Time, maxSpeed *float64, firstWrite *bool) {

	if err := f.Sync(); err != nil {
		fmt.Println("\nError syncing file:", err)
		return
	}

	totalElapsed := time.Since(writeStart)
	blockElapsed := time.Since(blockStart)

	blockSpeed := float64(blockWritten) / blockElapsed.Seconds()
	if blockSpeed > *maxSpeed {
		*maxSpeed = blockSpeed
	}
	avgSpeed := float64(totalWritten) / totalElapsed.Seconds()

	s := "" +
		"Written size  : " + padStr(formatBytes(totalWritten)) + "\n" +
		"Elapsed time  : " + padStr(formatDuration(totalElapsed)) + "\n" +
		"Average speed : " + padStr(formatBytes(int64(avgSpeed))) + "/s\n" +
		"Current speed : " + padStr(formatBytes(int64(blockSpeed))) + "/s\n" +
		"Max speed     : " + padStr(formatBytes(int64(*maxSpeed))) + "/s\n"

	if *firstWrite {
		*firstWrite = false
	} else {
		fmt.Printf("\033[%dA", strings.Count(s, "\n"))
	}
	fmt.Print(s)
}

func padStr(s string) string {
	return fmt.Sprintf("%12s", s)
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
