#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT="$ROOT/docs/assets/yuxin-demo.gif"
WORK=${TMPDIR:-/tmp}/yuxin-demo-render

command -v go >/dev/null 2>&1 || { echo "需要 Go" >&2; exit 1; }
command -v agg >/dev/null 2>&1 || { echo "需要 agg：brew install agg" >&2; exit 1; }

mkdir -p "$WORK"
cd "$ROOT"
go build -o "$WORK/session" scripts/demo-terminal-session.go

"$WORK/session" "$WORK/yuxin-demo.cast"

agg --quiet --theme github-dark --font-size 16 --line-height 1.35 \
  --fps-cap 12 --idle-time-limit 5 --last-frame-duration 3.2 --select event:0..100% \
  "$WORK/yuxin-demo.cast" "$OUTPUT"

echo "已生成：$OUTPUT"
