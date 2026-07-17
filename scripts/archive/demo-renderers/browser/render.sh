#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../../../.." && pwd)
OUTPUT=${1:-${TMPDIR:-/tmp}/yuxin-demo-browser-output}
WORK=${TMPDIR:-/tmp}/yuxin-demo-browser
CHROME=${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}

command -v go >/dev/null 2>&1 || { echo "需要 Go" >&2; exit 1; }
command -v ffmpeg >/dev/null 2>&1 || { echo "需要 ffmpeg" >&2; exit 1; }
[ -x "$CHROME" ] || { echo "找不到 Chrome，可通过 CHROME 指定路径" >&2; exit 1; }

mkdir -p "$OUTPUT" "$WORK"
cd "$ROOT"
go run scripts/archive/demo-renderers/browser/generate-data.go "$WORK/data.js"
cp "$ROOT/scripts/archive/demo-renderers/browser/render.html" "$WORK/index.html"

PAGE="file://$WORK/index.html"
for scene in intro dashboard share; do
  "$CHROME" --headless --disable-gpu --hide-scrollbars --allow-file-access-from-files \
    --force-device-scale-factor=1 --window-size=1440,900 \
    --screenshot="$WORK/$scene.png" "$PAGE?scene=$scene" >/dev/null 2>&1
done

ffmpeg -hide_banner -loglevel error -y \
  -loop 1 -t 2.2 -i "$WORK/intro.png" \
  -loop 1 -t 5.8 -i "$WORK/dashboard.png" \
  -loop 1 -t 3.4 -i "$WORK/share.png" \
  -filter_complex \
  "[0:v]fps=30,format=yuv420p[v0];[1:v]fps=30,format=yuv420p[v1];[2:v]fps=30,format=yuv420p[v2];[v0][v1]xfade=transition=fade:duration=0.45:offset=1.75[x1];[x1][v2]xfade=transition=fade:duration=0.45:offset=7.10,format=yuv420p[out]" \
  -map "[out]" -c:v libx264 -preset slow -crf 20 -movflags +faststart -an \
  "$OUTPUT/yuxin-demo-browser.mp4"

ffmpeg -hide_banner -loglevel error -y -i "$OUTPUT/yuxin-demo-browser.mp4" \
  -vf "fps=10,scale=960:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128:stats_mode=diff[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3:diff_mode=rectangle" \
  -loop 0 "$OUTPUT/yuxin-demo-browser.gif"

echo "已生成："
echo "  $OUTPUT/yuxin-demo-browser.mp4"
echo "  $OUTPUT/yuxin-demo-browser.gif"
