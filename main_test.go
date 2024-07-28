package main

import (
	"github.com/djherbis/times"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func createTestVideoFile(name string) (*os.File, error) {
	// Create a temporary file with a specific name format
	tempDir := os.TempDir()
	tempFilePath := tempDir + "/" + name + ".mp4"
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return nil, err
	}
	tempFile.Close()

	// Create a small dummy video file using ffmpeg
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=1280x720:rate=30", "-c:v", "libx264", tempFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		os.Remove(tempFilePath)
		return nil, err
	}

	return os.Open(tempFilePath)
}

func TestCheckRequirements(t *testing.T) {
	err := checkRequirements()
	if err != nil {
		t.Errorf("checkRequirements() error: %v", err)
	}
}

func TestGetFileTimes(t *testing.T) {
	// Create temporary files for testing
	tempFile1, err := createTestVideoFile("GH011234")
	if err != nil {
		t.Fatalf("Failed to create temp video file 1: %v", err)
	}
	defer os.Remove(tempFile1.Name())

	tempFile2, err := createTestVideoFile("GH021234")
	if err != nil {
		t.Fatalf("Failed to create temp video file 2: %v", err)
	}
	defer os.Remove(tempFile2.Name())

	// Set custom modification and creation times
	oldestTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	modTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

	os.Chtimes(tempFile1.Name(), oldestTime, oldestTime)
	os.Chtimes(tempFile2.Name(), modTime, modTime)

	inputPaths := []string{tempFile1.Name(), tempFile2.Name()}

	// Test getFileTimes function
	ct, mt, err := getFileTimes(inputPaths)
	if err != nil {
		t.Errorf("getFileTimes() error: %v", err)
	}

	if !ct.Equal(oldestTime) {
		t.Errorf("Expected oldest creation time %v, got %v", oldestTime, ct)
	}

	if !mt.Equal(modTime) {
		t.Errorf("Expected latest modification time %v, got %v", modTime, mt)
	}
}

func TestMergeFiles(t *testing.T) {
	// Create temporary output file and input files for testing
	tempFile1, err := createTestVideoFile("GH011234")
	if err != nil {
		t.Fatalf("Failed to create temp video file 1: %v", err)
	}
	defer os.Remove(tempFile1.Name())

	tempFile2, err := createTestVideoFile("GH021234")
	if err != nil {
		t.Fatalf("Failed to create temp video file 2: %v", err)
	}
	defer os.Remove(tempFile2.Name())

	outputFile, err := ioutil.TempFile("", "outputfile*.mp4")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer os.Remove(outputFile.Name())

	inputPaths := []string{tempFile1.Name(), tempFile2.Name()}
	creationTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	modTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Test mergeFiles function
	err = mergeFiles(outputFile.Name(), inputPaths, creationTime, modTime)
	if err != nil {
		t.Errorf("mergeFiles() error: %v", err)
	}

	// Check if the output file exists and has the correct timestamps
	info, err := os.Stat(outputFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	ts := times.Get(info)
	if !ts.HasBirthTime() || !ts.BirthTime().Equal(creationTime.In(time.Local)) {
		t.Errorf("Expected creation time %v, got %v", creationTime.In(time.Local), ts.BirthTime())
	}

	if !info.ModTime().Equal(modTime.In(time.Local)) {
		t.Errorf("Expected modification time %v, got %v", modTime, info.ModTime())
	}
}

func TestDuplicateFiles(t *testing.T) {
	// Create temporary output file and input files for testing
	tempFile1, err := createTestVideoFile("GH011234")
	if err != nil {
		t.Fatalf("Failed to create temp video file 1: %v", err)
	}
	defer os.Remove(tempFile1.Name())

	outputFile, err := ioutil.TempFile("", "outputfile*.mp4")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer os.Remove(outputFile.Name())

	inputPaths := []string{tempFile1.Name(), tempFile1.Name()} // Duplicate file
	creationTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	modTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Test mergeFiles function with duplicate files
	err = mergeFiles(outputFile.Name(), inputPaths, creationTime, modTime)
	if err == nil {
		t.Errorf("Expected error due to duplicate files, but got none")
	} else if !strings.Contains(err.Error(), "duplicate file detected") {
		t.Errorf("Expected error message to contain 'duplicate file detected', but got: %v", err)
	} else {
		t.Logf("Received expected error due to duplicate files: %v", err)
	}
}

func TestSingleFileMerge(t *testing.T) {
	// Create temporary output file and input file for testing
	tempFile, err := createTestVideoFile("GH011234")
	if err != nil {
		t.Fatalf("Failed to create temp video file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	outputFile, err := ioutil.TempFile("", "outputfile*.mp4")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer os.Remove(outputFile.Name())

	inputPaths := []string{tempFile.Name()}
	creationTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	modTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Test mergeFiles function with a single file
	err = mergeFiles(outputFile.Name(), inputPaths, creationTime, modTime)
	if err != nil {
		t.Errorf("mergeFiles() error: %v", err)
	}

	// Check if the output file is the same as the input file
	inputInfo, err := os.Stat(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat input file: %v", err)
	}

	outputInfo, err := os.Stat(outputFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	if inputInfo.Size() != outputInfo.Size() {
		t.Errorf("Expected output file size %v, got %v", inputInfo.Size(), outputInfo.Size())
	}

	if inputInfo.ModTime().Sub(outputInfo.ModTime()).Abs() > time.Second {
		t.Errorf("Expected output file modification time %v, got %v", inputInfo.ModTime(), outputInfo.ModTime())
	}
}
