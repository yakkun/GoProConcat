# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

GoProConcat is a macOS-only CLI that losslessly concatenates split GoPro video
files into one, preserving GoPro-specific metadata and matching the merged
file's creation/modification dates to the originals.

## Commands

```sh
go build -o GoProConcat main.go   # build
go test                           # run all tests
go test -run TestMergeFiles       # run a single test
go vet ./...                      # static checks
```

Run: `./GoProConcat outputfile inputfile1 [inputfile2 ...]`

Releases are tagged `vX.Y.Z` and published automatically by GoReleaser via
`.github/workflows/Release.yml` (darwin builds only, `CGO_ENABLED=0`).

## External tool dependencies (runtime)

The program shells out to external binaries that must exist on `PATH`;
`checkRequirements()` enforces them and the platform:
- `ffmpeg` — performs the actual stream copy / concat.
- `SetFile` — macOS Command Line Tools; sets the file's *creation* date (which
  `os.Chtimes` alone cannot do on macOS).
- The program refuses to run unless `runtime.GOOS == "darwin"`.

Tests require `ffmpeg` too — `createTestVideoFile` generates real `.mp4`
fixtures via `ffmpeg testsrc`, so tests fail without it installed.

## Architecture (single file: main.go)

The whole pipeline lives in `main.go`. Key design points worth knowing before
editing:

- **Filename-driven ordering.** GoPro names files `GH<chapter><file>.MP4` (AVC)
  or `GX...` (HEVC), e.g. `GH011234`, `GH021234`. `parseFileName` extracts the
  2-digit chapter and 4-digit file number via regex; `mergeFiles` sorts by
  *file number first, then chapter number* so chapters of the same recording
  join in order. Don't assume CLI argument order is the merge order.

- **Lossless concat, not re-encode.** ffmpeg is invoked with the `concat`
  demuxer + `-c copy`. The mapping flags are deliberate and preserve GoPro
  telemetry: `-map 0:v -map 0:a? -map 0:3? -copy_unknown -tag:2 gpmd`. The
  `gpmd` stream (GPS/gyro metadata) is the reason for `-copy_unknown` and the
  stream-3 mapping — changing these silently drops telemetry.

- **Timestamp preservation is three-step.** `getFileTimes` finds the *oldest*
  birth time (via `djherbis/times`) and the *newest* mod time across inputs.
  After merging, the creation date is written with `SetFile -d`, then
  `os.Chtimes` sets access/mod times. Both are needed.

- **Single-input shortcut.** If exactly one input is given, `mergeFiles` just
  copies the file (no ffmpeg, no metadata work) — see `copyFile`.

- **Duplicate guard.** Inputs are resolved to absolute paths and rejected if any
  path repeats.
