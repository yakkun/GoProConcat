package main

import (
	"fmt"
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

	for _, inputPath := range inputPaths {
		absPath, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %v", inputPath, err)
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
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(listFile.Name())

	for _, file := range files {
		_, err = listFile.WriteString(fmt.Sprintf("file '%s'\n", file.Path))
		if err != nil {
			return fmt.Errorf("failed to write to temp file: %v", err)
		}
	}
	listFile.Close()

	cmd := exec.Command(
		"ffmpeg",
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
		return fmt.Errorf("ffmpeg command failed: %v", err)
	}

	fmt.Printf("Setting creation time using SetFile: %s\n", creationTime.In(time.Local).Format("01/02/2006 15:04:05"))
	cmd = exec.Command("SetFile", "-d", creationTime.In(time.Local).Format("01/02/2006 15:04:05"), outputPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set creation time for %s: %v", outputPath, err)
	}

	err = os.Chtimes(outputPath, creationTime, modTime)
	if err != nil {
		return fmt.Errorf("failed to set file times for %s: %v", outputPath, err)
	}

	return nil
}

func getFileTimes(inputPaths []string) (time.Time, time.Time, error) {
	var oldestTime time.Time
	var modTime time.Time
	for _, inputPath := range inputPaths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to stat input file %s: %v", inputPath, err)
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

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: GoProConcat outputfile inputfile1 [inputfile2 ...]")
		return
	}

	err := checkRequirements()
	if err != nil {
		fmt.Println(err)
		return
	}

	outputPath := os.Args[1]
	inputPaths := os.Args[2:]

	creationTime, modTime, err := getFileTimes(inputPaths)
	if err != nil {
		fmt.Printf("Error getting file times: %v\n", err)
		return
	}

	err = mergeFiles(outputPath, inputPaths, creationTime, modTime)
	if err != nil {
		fmt.Printf("Error merging files: %v\n", err)
		return
	}

	fmt.Println("Files merged successfully")
}
