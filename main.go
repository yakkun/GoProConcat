package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/djherbis/times"
)

type FileInfo struct {
	Path          string
	FileNumber    int
	ChapterNumber int
}

func checkRequirements() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("this program is designed to run on macOS")
	}

	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg is not installed. Please install it using Homebrew:\n\nbrew install ffmpeg")
	}

	_, err = exec.LookPath("SetFile")
	if err != nil {
		return fmt.Errorf("SetFile is not installed. Please install Command Line Tools:\n\nxcode-select --install")
	}

	return nil
}

func parseFileName(filePath string) (FileInfo, error) {
	re := regexp.MustCompile(`(GH|GX)(\d{2})(\d{4})\.(?i:mp4)`)
	matches := re.FindStringSubmatch(strings.ToUpper(filepath.Base(filePath)))
	if len(matches) < 4 {
		return FileInfo{}, fmt.Errorf("invalid file format: %s", filePath)
	}
	chapterNumber, _ := strconv.Atoi(matches[2])
	fileNumber, _ := strconv.Atoi(matches[3])
	return FileInfo{
		Path:          filePath,
		FileNumber:    fileNumber,
		ChapterNumber: chapterNumber,
	}, nil
}

func mergeFiles(outputPath string, inputPaths []string, creationTime, modTime time.Time) error {
	var files []FileInfo
	fileMap := make(map[string]bool)

	if len(inputPaths) == 1 {
		return copyFile(inputPaths[0], outputPath)
	}

	for _, inputPath := range inputPaths {
		absPath, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", inputPath, err)
		}

		if fileMap[absPath] {
			return fmt.Errorf("duplicate file detected: %s. Please remove duplicates and try again", absPath)
		}
		fileMap[absPath] = true

		fileInfo, err := parseFileName(inputPath)
		if err != nil {
			return err
		}
		fileInfo.Path = absPath
		files = append(files, fileInfo)
	}

	// Sort files by FileNumber and ChapterNumber
	sort.Slice(files, func(i, j int) bool {
		if files[i].FileNumber == files[j].FileNumber {
			return files[i].ChapterNumber < files[j].ChapterNumber
		}
		return files[i].FileNumber < files[j].FileNumber
	})

	listFile, err := os.CreateTemp("", "*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(listFile.Name())

	for _, file := range files {
		_, err = listFile.WriteString(fmt.Sprintf("file '%s'\n", file.Path))
		if err != nil {
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
	}
	listFile.Close()

	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner", "-nostats", "-loglevel", "error", // Suppress FFmpeg output
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",
		"-y",
		"-map", "0:v",
		"-map", "0:a?",
		"-map", "0:3?",
		"-copy_unknown",
		"-tag:2", "gpmd",
		"-metadata", fmt.Sprintf("creation_time=%s", creationTime.Format(time.RFC3339)),
		outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	fmt.Printf("Setting creation time using SetFile: %s\n", creationTime.In(time.Local).Format("01/02/2006 15:04:05"))
	cmd = exec.Command("SetFile", "-d", creationTime.In(time.Local).Format("01/02/2006 15:04:05"), outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set creation time for %s: %w", outputPath, err)
	}

	err = os.Chtimes(outputPath, creationTime, modTime)
	if err != nil {
		return fmt.Errorf("failed to set file times for %s: %w", outputPath, err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return err
	}

	return destinationFile.Close()
}

func getFileTimes(inputPaths []string) (time.Time, time.Time, error) {
	var oldestTime time.Time
	var modTime time.Time
	for _, inputPath := range inputPaths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to stat input file %s: %w", inputPath, err)
		}

		ts := times.Get(info)
		if ts.HasBirthTime() && (oldestTime.IsZero() || ts.BirthTime().Before(oldestTime)) {
			oldestTime = ts.BirthTime()
		}
		if modTime.Before(info.ModTime()) {
			modTime = info.ModTime()
		}
	}

	if oldestTime.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to get oldest creation time")
	}

	return oldestTime, modTime, nil
}

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: GoProConcat outputfile inputfile1 [inputfile2 ...]")
	}

	if err := checkRequirements(); err != nil {
		return err
	}

	outputPath := os.Args[1]
	inputPaths := os.Args[2:]

	creationTime, modTime, err := getFileTimes(inputPaths)
	if err != nil {
		return fmt.Errorf("getting file times: %w", err)
	}

	if err := mergeFiles(outputPath, inputPaths, creationTime, modTime); err != nil {
		return fmt.Errorf("merging files: %w", err)
	}

	fmt.Println("Files merged successfully")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
