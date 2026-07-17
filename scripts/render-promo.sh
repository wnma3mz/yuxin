#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT=${1:-${TMPDIR:-/tmp}/yuxin-promo-review.mp4}
FONT=${YUXIN_PROMO_FONT:-/System/Library/Fonts/STHeiti Medium.ttc}

command -v ffmpeg >/dev/null 2>&1 || { echo "需要 ffmpeg" >&2; exit 1; }
[ -f "$FONT" ] || { echo "找不到中文字体，可通过 YUXIN_PROMO_FONT 指定路径" >&2; exit 1; }

"$ROOT/scripts/render-demo.sh"
mkdir -p "$(dirname -- "$OUTPUT")"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i "color=c=0x07101f:s=1920x1080:r=30:d=3.4" \
  -ignore_loop 1 -i "$ROOT/docs/assets/yuxin-demo.gif" \
  -f lavfi -i "color=c=0x07101f:s=1920x1080:r=30:d=4.5" \
  -filter_complex "
    [0:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='这班不能白上':fontcolor=0xf8fafc:fontsize=76:x=(w-text_w)/2:y=325,
      drawtext=fontfile='$FONT':text='至少得知道赚了多少':fontcolor=0x4ade80:fontsize=48:x=(w-text_w)/2:y=465,
      drawtext=fontfile='$FONT':text='YUXIN · 余薪':fontcolor=0x94a3b8:fontsize=30:x=(w-text_w)/2:y=590,
      fade=t=out:st=3.05:d=0.35:color=0x07101f[intro];
    [1:v]fps=30,settb=AVTB,split=2[dashboard_source][share_source];
    [dashboard_source]trim=start=0:end=5,setpts=PTS-STARTPTS,
      tpad=stop_mode=clone:stop_duration=4,
      crop=iw:520:0:0,scale=1660:-2:flags=lanczos,
      pad=1920:1080:(ow-iw)/2:190:color=0x07101f,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0x4ade80:fontsize=50:x=(w-text_w)/2:y=55,
      drawtext=fontfile='$FONT':text='今日入账 · 下班倒计时 · 工作日进度 · 退休倒计时':fontcolor=0x94a3b8:fontsize=27:x=(w-text_w)/2:y=115,
      fade=t=in:st=0:d=0.4:color=0x07101f,fade=t=out:st=8.6:d=0.4:color=0x07101f[dashboard];
    [share_source]trim=start=8:end=11.1,setpts=PTS-STARTPTS,
      crop=iw:500:0:0,scale=1600:-2:flags=lanczos,
      pad=1920:1080:(ow-iw)/2:190:color=0x07101f,
      drawtext=fontfile='$FONT':text='想晒进度，一键生成分享卡':fontcolor=0xf8fafc:fontsize=46:x=(w-text_w)/2:y=55,
      drawtext=fontfile='$FONT':text='默认使用演示数据':fontcolor=0x94a3b8:fontsize=27:x=(w-text_w)/2:y=113,
      fade=t=in:st=0:d=0.4:color=0x07101f,fade=t=out:st=2.7:d=0.4:color=0x07101f[share];
    [2:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='YUXIN · 余薪':fontcolor=0x4ade80:fontsize=74:x=(w-text_w)/2:y=270,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0xf8fafc:fontsize=48:x=(w-text_w)/2:y=410,
      drawtext=fontfile='$FONT':text='开源 · 本地运行 · macOS / Windows / Linux':fontcolor=0xdbe5f2:fontsize=30:x=(w-text_w)/2:y=545,
      drawtext=fontfile='$FONT':text='github.com/wnma3mz/yuxin':fontcolor=0x94a3b8:fontsize=30:x=(w-text_w)/2:y=650,
      fade=t=in:st=0:d=0.4:color=0x07101f[outro];
    [intro][dashboard][share][outro]concat=n=4:v=1:a=0[out]
  " \
  -map "[out]" -c:v libx264 -preset slow -crf 20 -pix_fmt yuv420p \
  -movflags +faststart -an "$OUTPUT"

echo "已生成：$OUTPUT"
