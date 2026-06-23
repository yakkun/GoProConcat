#!/usr/bin/env zsh
#
# concat-dir.sh — group GoPro chapter files in a directory by file number and
# concatenate each multi-chapter session with GoProConcat.
#
# Usage:
#   concat-dir.sh <directory> [--run]
#
# Without --run it only prints the plan (dry-run). The merged file for each
# session is written into the same directory using chapter number 00 (e.g.
# GX002311.MP4), which GoPro never uses, so it does not collide with the
# original chapters. Chapter 00 files are treated as already-merged outputs and
# excluded from inputs, making re-runs safe.

set -euo pipefail

SCRIPT_DIR=${0:A:h}
BIN=${GOPROCONCAT_BIN:-$SCRIPT_DIR/../GoProConcat}

if [[ $# -lt 1 ]]; then
  print -u2 "Usage: concat-dir.sh <directory> [--run]"
  exit 1
fi

DIR=$1
RUN=0
[[ ${2:-} == "--run" ]] && RUN=1

if [[ ! -d $DIR ]]; then
  print -u2 "Not a directory: $DIR"
  exit 1
fi
if [[ ! -x $BIN ]]; then
  print -u2 "GoProConcat binary not found/executable: $BIN"
  exit 1
fi

OUTDIR=$DIR

# Group chapters by 4-digit file number. Chapter 00 is reserved for merged
# outputs, so skip it as an input.
typeset -A sessions
for f in $DIR/*.MP4(N) $DIR/*.mp4(N); do
  base=${f:t}
  if [[ ${base:u} =~ '^(GH|GX)([0-9]{2})([0-9]{4})\.MP4$' ]]; then
    prefix=$match[1]
    chap=$match[2]
    num=$match[3]
    [[ $chap == "00" ]] && continue
    sessions[$num]+="${chap}|${prefix}|${f}"$'\n'
  fi
done

if [[ ${#sessions} -eq 0 ]]; then
  print "No GoPro files found in $DIR"
  exit 0
fi

print "Directory: $DIR"
print "Output:    $OUTDIR"
(( RUN )) && print "Mode:      RUN" || print "Mode:      dry-run (pass --run to execute)"
print ""

(( RUN )) && mkdir -p $OUTDIR

merge_count=0
for num in ${(ok)sessions}; do
  # Sort this session's chapters numerically and collect paths in order.
  lines=( ${(f)sessions[$num]} )
  sorted=( ${(on)lines} )
  paths=()
  prefix=""
  for line in $sorted; do
    parts=( ${(s:|:)line} )
    prefix=$parts[2]
    paths+=( $parts[3] )
  done
  count=${#paths}
  if (( count < 2 )); then
    print "  ${prefix}${num}: single chapter — skip"
    continue
  fi
  out=$OUTDIR/${prefix}00${num}.MP4
  print "  ${prefix}${num}: ${count} chapters -> ${out:t}"
  for p in $paths; do print "      ${p:t}"; done
  merge_count=$(( merge_count + 1 ))
  if (( RUN )); then
    print "    >> merging..."
    "$BIN" "$out" $paths
  fi
done

print ""
print "Sessions to merge: ${merge_count}"
if (( ! RUN )); then
  print "Dry-run only. Re-run with --run to execute."
else
  print "Done. Output in $OUTDIR"
fi
