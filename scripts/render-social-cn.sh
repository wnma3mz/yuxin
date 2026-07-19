#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT=${1:-${TMPDIR:-/tmp}/yuxin-social-cn.mp4}
FONT=${YUXIN_SOCIAL_FONT:-/System/Library/Fonts/STHeiti Medium.ttc}
VOICE_TRACK=${YUXIN_SOCIAL_VOICE_TRACK:-$ROOT/docs/assets/yuxin-promo-voice.m4a}
FFMPEG=${YUXIN_FFMPEG:-ffmpeg}
PAGE_URL=${YUXIN_SOCIAL_PAGE_URL:-}
CHROME=${YUXIN_CHROME:-}

command -v "$FFMPEG" >/dev/null 2>&1 || { echo "需要 ffmpeg" >&2; exit 1; }
if ! "$FFMPEG" -filters 2>/dev/null | grep -q ' drawtext '; then
  if [ -x /opt/homebrew/opt/ffmpeg-full/bin/ffmpeg ]; then
    FFMPEG=/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg
  else
    echo "当前 ffmpeg 缺少 drawtext 滤镜；可通过 YUXIN_FFMPEG 指定完整版本" >&2
    exit 1
  fi
fi
[ -f "$FONT" ] || { echo "找不到中文字体，可通过 YUXIN_SOCIAL_FONT 指定" >&2; exit 1; }
[ -f "$VOICE_TRACK" ] || { echo "找不到中文配音音轨" >&2; exit 1; }

if [ -z "$CHROME" ]; then
  for candidate in \
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' \
    '/Applications/Chromium.app/Contents/MacOS/Chromium' \
    google-chrome chromium chromium-browser; do
    if [ -x "$candidate" ]; then
      CHROME=$candidate
      break
    fi
    if command -v "$candidate" >/dev/null 2>&1; then
      CHROME=$(command -v "$candidate")
      break
    fi
  done
fi
if [ -z "$CHROME" ] || [ ! -x "$CHROME" ]; then
  echo "找不到 Chrome/Chromium，可通过 YUXIN_CHROME 指定" >&2
  exit 1
fi

mkdir -p "$(dirname -- "$OUTPUT")"
TEMPORARY=$(mktemp -d "${TMPDIR:-/tmp}/yuxin-social-cn.XXXXXX")
SERVER_PID=
cleanup() {
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TEMPORARY"
}
trap cleanup EXIT HUP INT TERM

if [ -z "$PAGE_URL" ]; then
  VITE=$ROOT/web/node_modules/.bin/vite
  [ -x "$VITE" ] || {
    echo "缺少 Web 依赖，请先在 web/ 运行 npm ci" >&2
    exit 1
  }
  PORT=${YUXIN_SOCIAL_PORT:-4175}
  PAGE_URL="http://127.0.0.1:$PORT/"
  (
    cd "$ROOT/web"
    exec env VITE_USE_MOCK_DATA=false "$VITE" --host 127.0.0.1 --port "$PORT"
  ) >"$TEMPORARY/vite.log" 2>&1 &
  SERVER_PID=$!
  ready=0
  attempt=0
  while [ "$attempt" -lt 50 ]; do
    if curl -fsS "$PAGE_URL" >/dev/null 2>&1; then
      ready=1
      break
    fi
    attempt=$((attempt + 1))
    sleep 0.2
  done
  if [ "$ready" -ne 1 ]; then
    cat "$TEMPORARY/vite.log" >&2
    echo "本地 Web 页面启动失败" >&2
    exit 1
  fi
fi

WEB_CAPTURE="$TEMPORARY/yuxin-web.png"
"$CHROME" \
  --headless=new \
  --disable-gpu \
  --hide-scrollbars \
  --force-device-scale-factor=2 \
  --window-size=540,2100 \
  --virtual-time-budget=5000 \
  --screenshot="$WEB_CAPTURE" \
  "$PAGE_URL" >"$TEMPORARY/chrome.log" 2>&1
[ -s "$WEB_CAPTURE" ] || {
  cat "$TEMPORARY/chrome.log" >&2
  echo "浏览器画面截取失败" >&2
  exit 1
}

VISUAL="$TEMPORARY/visual.mp4"
"$FFMPEG" -hide_banner -loglevel error -y \
  -loop 1 -framerate 30 -t 3.4 -i "$WEB_CAPTURE" \
  -loop 1 -framerate 30 -t 9 -i "$WEB_CAPTURE" \
  -loop 1 -framerate 30 -t 3.1 -i "$WEB_CAPTURE" \
  -loop 1 -framerate 30 -t 4.5 -i "$WEB_CAPTURE" \
  -filter_complex "
    [0:v]crop=1080:1920:0:0,fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      fade=t=in:st=0:d=0.25:color=0x07100e,fade=t=out:st=3.05:d=0.35:color=0x07100e[intro];
    [1:v]crop=1080:1920:0:'min(1050\,t*116.667)',fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawbox=x=0:y=1715:w=1080:h=205:color=0x07100e@0.82:t=fill,
      drawtext=fontfile='$FONT':text='今天赚了多少  ·  还有多久下班':fontcolor=0xe4e4e7:fontsize=38:x=(w-text_w)/2:y=1760,
      drawtext=fontfile='$FONT':text='本地保存自己的  ·  匿名查看大家的':fontcolor=0x34d399:fontsize=29:x=(w-text_w)/2:y=1830,
      fade=t=in:st=0:d=0.35:color=0x07100e,fade=t=out:st=8.6:d=0.4:color=0x07100e[dashboard];
    [2:v]crop=1080:1920:0:'1800+min(480\,t*154.839)',fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawbox=x=0:y=1715:w=1080:h=205:color=0x07100e@0.84:t=fill,
      drawtext=fontfile='$FONT':text='公开样本只展示匿名聚合结果':fontcolor=0x38bdf8:fontsize=38:x=(w-text_w)/2:y=1765,
      drawtext=fontfile='$FONT':text='不展示作者  ·  不公开个人原始记录':fontcolor=0xa1a1aa:fontsize=28:x=(w-text_w)/2:y=1835,
      fade=t=in:st=0:d=0.35:color=0x07100e,fade=t=out:st=2.7:d=0.4:color=0x07100e[public];
    [3:v]crop=1080:1920:0:0,gblur=sigma=10,eq=brightness=-0.3,fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawbox=x=70:y=430:w=940:h=940:color=0x07100e@0.78:t=fill,
      drawbox=x=70:y=430:w=940:h=940:color=0x34d399@0.45:t=2,
      drawtext=fontfile='$FONT':text='YUXIN  ·  余薪':fontcolor=0x34d399:fontsize=92:x=(w-text_w)/2:y=560,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0xe4e4e7:fontsize=54:x=(w-text_w)/2:y=720,
      drawtext=fontfile='$FONT':text='网页版':fontcolor=0xa1a1aa:fontsize=30:x=(w-text_w)/2:y=920,
      drawtext=fontfile='$FONT':text='wnma3mz.github.io/yuxin/':fontcolor=0x38bdf8:fontsize=34:x=(w-text_w)/2:y=985,
      drawtext=fontfile='$FONT':text='开源项目':fontcolor=0xa1a1aa:fontsize=30:x=(w-text_w)/2:y=1120,
      drawtext=fontfile='$FONT':text='github.com/wnma3mz/yuxin':fontcolor=0xa7f3d0:fontsize=31:x=(w-text_w)/2:y=1185,
      fade=t=in:st=0:d=0.35:color=0x07100e[outro];
    [intro][dashboard][public][outro]concat=n=4:v=1:a=0[out]
  " \
  -map "[out]" -c:v libx264 -preset slow -crf 18 -pix_fmt yuv420p \
  -movflags +faststart -an "$VISUAL"

"$FFMPEG" -hide_banner -loglevel error -y \
  -i "$VISUAL" -i "$VOICE_TRACK" \
  -map 0:v -map 1:a -c:v copy -c:a copy -movflags +faststart -shortest "$OUTPUT"

echo "已生成浏览器界面的中文版竖屏推广视频：$OUTPUT"
