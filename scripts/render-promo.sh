#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT=${1:-${TMPDIR:-/tmp}/yuxin-promo-review.mp4}
FONT=${YUXIN_PROMO_FONT:-/System/Library/Fonts/STHeiti Medium.ttc}
VOICE_TRACK=${YUXIN_PROMO_VOICE_TRACK:-$ROOT/docs/assets/yuxin-promo-voice.m4a}
SILENT=${YUXIN_PROMO_SILENT:-0}
FFMPEG=${YUXIN_FFMPEG:-ffmpeg}

command -v "$FFMPEG" >/dev/null 2>&1 || { echo "需要 ffmpeg" >&2; exit 1; }
if ! "$FFMPEG" -filters 2>/dev/null | grep -q ' drawtext '; then
  if [ -x /opt/homebrew/opt/ffmpeg-full/bin/ffmpeg ]; then
    FFMPEG=/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg
  else
    echo "当前 ffmpeg 缺少 drawtext 滤镜；macOS 可安装 ffmpeg-full，或通过 YUXIN_FFMPEG 指定完整版本" >&2
    exit 1
  fi
fi
[ -f "$FONT" ] || { echo "找不到中文字体，可通过 YUXIN_PROMO_FONT 指定路径" >&2; exit 1; }
[ "$SILENT" = "1" ] || [ -f "$VOICE_TRACK" ] || {
  echo "找不到配音音轨，可通过 YUXIN_PROMO_VOICE_TRACK 指定路径" >&2
  exit 1
}

"$ROOT/scripts/render-demo.sh"
mkdir -p "$(dirname -- "$OUTPUT")"
TEMPORARY=$(mktemp -d "${TMPDIR:-/tmp}/yuxin-promo.XXXXXX")
trap 'rm -rf "$TEMPORARY"' EXIT HUP INT TERM
VISUAL="$TEMPORARY/visual.mp4"

"$FFMPEG" -hide_banner -loglevel error -y \
  -f lavfi -i "color=c=0x0a1211:s=1920x1080:r=30:d=3.4" \
  -ignore_loop 1 -i "$ROOT/docs/assets/yuxin-demo.gif" \
  -f lavfi -i "color=c=0x0a1211:s=1920x1080:r=30:d=3.1" \
  -f lavfi -i "color=c=0x0a1211:s=1920x1080:r=30:d=4.5" \
  -filter_complex "
    [0:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='这班不能白上':fontcolor=0xe4e4e7:fontsize=76:x=(w-text_w)/2:y=325,
      drawtext=fontfile='$FONT':text='至少得知道赚了多少':fontcolor=0x34d399:fontsize=48:x=(w-text_w)/2:y=465,
      drawtext=fontfile='$FONT':text='YUXIN · 余薪':fontcolor=0xa1a1aa:fontsize=30:x=(w-text_w)/2:y=590,
      fade=t=out:st=3.05:d=0.35:color=0x0a1211[intro];
    [1:v]fps=30,settb=AVTB,trim=start=0:end=9,setpts=PTS-STARTPTS,
      crop=iw:520:0:0,scale=1660:-2:flags=lanczos,
      pad=1920:1080:(ow-iw)/2:190:color=0x0a1211,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0x34d399:fontsize=50:x=(w-text_w)/2:y=55,
      drawtext=fontfile='$FONT':text='今日入账 · 下班倒计时 · 工作日进度 · 退休倒计时':fontcolor=0xa1a1aa:fontsize=27:x=(w-text_w)/2:y=115,
      fade=t=in:st=0:d=0.4:color=0x0a1211,fade=t=out:st=8.6:d=0.4:color=0x0a1211[dashboard];
    [2:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='不想打开终端？':fontcolor=0xa1a1aa:fontsize=30:x=(w-text_w)/2:y=245,
      drawtext=fontfile='$FONT':text='网页版也能先算':fontcolor=0xe4e4e7:fontsize=72:x=(w-text_w)/2:y=330,
      drawtext=fontfile='$FONT':text='本地保存自己的 · 匿名查看大家的':fontcolor=0x34d399:fontsize=36:x=(w-text_w)/2:y=465,
      drawbox=x=510:y=590:w=900:h=92:color=0x101c1a@0.95:t=fill,
      drawbox=x=510:y=590:w=900:h=92:color=0x38bdf8@0.45:t=2,
      drawtext=fontfile='$FONT':text='wnma3mz.github.io/yuxin/':fontcolor=0x38bdf8:fontsize=36:x=(w-text_w)/2:y=617,
      drawtext=fontfile='$FONT':text='无需登录 · 不生成浏览器指纹':fontcolor=0xa1a1aa:fontsize=26:x=(w-text_w)/2:y=745,
      fade=t=in:st=0:d=0.35:color=0x0a1211,fade=t=out:st=2.7:d=0.4:color=0x0a1211[web];
    [3:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='YUXIN · 余薪':fontcolor=0x34d399:fontsize=74:x=(w-text_w)/2:y=270,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0xe4e4e7:fontsize=48:x=(w-text_w)/2:y=410,
      drawtext=fontfile='$FONT':text='开源 · 本地运行 · macOS / Windows / Linux':fontcolor=0xd4d4d8:fontsize=30:x=(w-text_w)/2:y=545,
      drawtext=fontfile='$FONT':text='github.com/wnma3mz/yuxin':fontcolor=0xa1a1aa:fontsize=28:x=(w-text_w)/2:y=635,
      drawtext=fontfile='$FONT':text='wnma3mz.github.io/yuxin/':fontcolor=0x38bdf8:fontsize=28:x=(w-text_w)/2:y=690,
      fade=t=in:st=0:d=0.4:color=0x0a1211[outro];
    [intro][dashboard][web][outro]concat=n=4:v=1:a=0[out]
  " \
  -map "[out]" -c:v libx264 -preset slow -crf 20 -pix_fmt yuv420p \
  -movflags +faststart -an "$VISUAL"

if [ "$SILENT" = "1" ]; then
  mv "$VISUAL" "$OUTPUT"
  echo "已生成无声版：$OUTPUT"
  exit 0
fi

"$FFMPEG" -hide_banner -loglevel error -y \
  -i "$VISUAL" -i "$VOICE_TRACK" \
  -map 0:v -map 1:a \
  -c:v copy -c:a copy -movflags +faststart -shortest "$OUTPUT"

echo "已生成：$OUTPUT"
