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
  -f lavfi -i "color=c=0x07101f:s=1920x1080:r=30:d=2.6" \
  -ignore_loop 1 -i "$ROOT/docs/assets/yuxin-demo.gif" \
  -f lavfi -i "color=c=0x07101f:s=1920x1080:r=30:d=4.5" \
  -filter_complex "
    [0:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='YUXIN':fontcolor=0xf8fafc:fontsize=104:x=(w-text_w)/2:y=330,
      drawtext=fontfile='$FONT':text='余薪':fontcolor=0xdbe5f2:fontsize=52:x=(w-text_w)/2:y=455,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期。':fontcolor=0x4ade80:fontsize=44:x=(w-text_w)/2:y=565,
      drawtext=fontfile='$FONT':text='本地离线运行的工作仪表盘':fontcolor=0x94a3b8:fontsize=28:x=(w-text_w)/2:y=650,
      fade=t=out:st=2.2:d=0.4:color=0x07101f[intro];
    [1:v]fps=30,settb=AVTB,scale=1500:-2:flags=lanczos,
      pad=1920:1080:(ow-iw)/2:145:color=0x07101f,
      drawtext=fontfile='$FONT':text='今日入账 · 下班倒计时 · 节假日进度':fontcolor=0xf8fafc:fontsize=38:x=(w-text_w)/2:y=70:enable='between(t,0,5)',
      drawtext=fontfile='$FONT':text='截图前隐藏金额和存款，真实数据留在本地':fontcolor=0xf8fafc:fontsize=38:x=(w-text_w)/2:y=70:enable='between(t,5,8)',
      drawtext=fontfile='$FONT':text='固定合成数据分享，不暴露个人信息':fontcolor=0xf8fafc:fontsize=38:x=(w-text_w)/2:y=70:enable='between(t,8,13)',
      trim=duration=13,setpts=PTS-STARTPTS,
      fade=t=in:st=0:d=0.4:color=0x07101f,fade=t=out:st=12.6:d=0.4:color=0x07101f[demo];
    [2:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawtext=fontfile='$FONT':text='无账号 · 离线运行 · 数据只在本地':fontcolor=0x4ade80:fontsize=44:x=(w-text_w)/2:y=300,
      drawtext=fontfile='$FONT':text='一行命令即可开始':fontcolor=0xdbe5f2:fontsize=30:x=(w-text_w)/2:y=405,
      drawtext=fontfile='$FONT':text='curl -fsSL https\\://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.sh | sh':fontcolor=0xf8fafc:fontsize=27:x=(w-text_w)/2:y=505:box=1:boxcolor=0x111c2d@0.95:boxborderw=24,
      drawtext=fontfile='$FONT':text='github.com/wnma3mz/yuxin':fontcolor=0x94a3b8:fontsize=28:x=(w-text_w)/2:y=650,
      fade=t=in:st=0:d=0.4:color=0x07101f[outro];
    [intro][demo][outro]concat=n=3:v=1:a=0[out]
  " \
  -map "[out]" -c:v libx264 -preset slow -crf 20 -pix_fmt yuv420p \
  -movflags +faststart -an "$OUTPUT"

echo "已生成：$OUTPUT"
