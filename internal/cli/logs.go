package cli

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"cs-cloud/internal/app"
)

func logs(a *app.App) error {
	path := a.LogFilePath()
	f, err := os.Open(path)
	if err != nil {
		printWarn("No log file found")
		printInfo("Path: %s", path)
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := len(lines) - 100
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return nil
}

func logf(a *app.App) error {
	tailLogs(a, true)
	return nil
}

func tailLogs(a *app.App, follow bool) {
	path := a.LogFilePath()
	if _, err := os.Stat(path); err != nil {
		printWarn("No log file found")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	f.Close()

	start := len(lines) - 100
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	if !follow {
		return
	}

	info, _ := os.Stat(path)
	if info == nil {
		return
	}
	size := info.Size()

	for {
		time.Sleep(500 * time.Millisecond)
		info, err := os.Stat(path)
		if err != nil || info.Size() <= size {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		f.Seek(size, 0)
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		info2, _ := f.Stat()
		if info2 != nil {
			size = info2.Size()
		}
		f.Close()
	}
}
