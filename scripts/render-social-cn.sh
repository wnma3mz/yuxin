#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT=${1:-${TMPDIR:-/tmp}/yuxin-social-cn.mp4}
FONT=${YUXIN_SOCIAL_FONT:-/System/Library/Fonts/STHeiti Medium.ttc}
VOICE_TRACK=${YUXIN_SOCIAL_VOICE_TRACK:-$ROOT/docs/assets/yuxin-promo-voice.m4a}
FFMPEG=${YUXIN_FFMPEG:-ffmpeg}

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

"$ROOT/scripts/render-demo.sh"
mkdir -p "$(dirname -- "$OUTPUT")"
TEMPORARY=$(mktemp -d "${TMPDIR:-/tmp}/yuxin-social-cn.XXXXXX")
trap 'rm -rf "$TEMPORARY"' EXIT HUP INT TERM
VISUAL="$TEMPORARY/visual.mp4"

"$FFMPEG" -hide_banner -loglevel error -y \
  -f lavfi -i "color=c=0x0a1211:s=1080x1920:r=30:d=3.4" \
  -ignore_loop 1 -i "$ROOT/docs/assets/yuxin-demo.gif" \
  -f lavfi -i "color=c=0x0a1211:s=1080x1920:r=30:d=3.1" \
  -f lavfi -i "color=c=0x0a1211:s=1080x1920:r=30:d=4.5" \
  -filter_complex "
    [0:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawgrid=w=90:h=90:t=1:c=0x34d399@0.055,
      drawtext=fontfile='$FONT':text='YUXIN  /  余薪':fontcolor=0x34d399:fontsize=38:x=(w-text_w)/2:y=250,
      drawtext=fontfile='$FONT':text='这班不能白上':fontcolor=0xe4e4e7:fontsize=92:x=(w-text_w)/2:y=610,
      drawtext=fontfile='$FONT':text='至少得知道赚了多少':fontcolor=0xfbbf24:fontsize=56:x=(w-text_w)/2:y=770,
      drawtext=fontfile='$FONT':text='+¥0.03':fontcolor=0x38bdf8:fontsize=84:x=(w-text_w)/2:y=980:alpha='0.45+0.55*abs(sin(PI*t))',
      drawtext=fontfile='$FONT':text='一个按秒计算工资的本地终端工具':fontcolor=0xa1a1aa:fontsize=32:x=(w-text_w)/2:y=1250,
      drawtext=fontfile='$FONT':text='开源  ·  无账号  ·  数据不上云':fontcolor=0xd4d4d8:fontsize=28:x=(w-text_w)/2:y=1430,
      fade=t=in:st=0:d=0.25:color=0x0a1211,fade=t=out:st=3.05:d=0.35:color=0x0a1211[intro];
    [1:v]fps=30,settb=AVTB,trim=start=0:end=9,setpts=PTS-STARTPTS,
      crop=iw:520:0:0,scale=1000:-2:flags=lanczos,
      pad=1080:1920:(ow-iw)/2:430:color=0x0a1211,
      drawgrid=w=90:h=90:t=1:c=0x34d399@0.04,
      drawtext=fontfile='$FONT':text='工资到账，是可以看见的':fontcolor=0xe4e4e7:fontsize=61:x=(w-text_w)/2:y=175,
      drawtext=fontfile='$FONT':text='¥521.38':fontcolor=0xfbbf24:fontsize=94:x=100:y=1080,
      drawtext=fontfile='$FONT':text='距离下班  2h 35m':fontcolor=0x38bdf8:fontsize=48:x=100:y=1210,
      drawbox=x=85:y=1360:w=250:h=72:color=0x34d399@0.09:t=fill,
      drawbox=x=365:y=1360:w=250:h=72:color=0x38bdf8@0.09:t=fill,
      drawbox=x=645:y=1360:w=250:h=72:color=0xfbbf24@0.09:t=fill,
      drawtext=fontfile='$FONT':text='本地运行':fontcolor=0xa7f3d0:fontsize=28:x=150:y=1378,
      drawtext=fontfile='$FONT':text='离线计算':fontcolor=0x7dd3fc:fontsize=28:x=430:y=1378,
      drawtext=fontfile='$FONT':text='隐私可控':fontcolor=0xfcd34d:fontsize=28:x=710:y=1378,
      drawtext=fontfile='$FONT':text='今天赚了多少  ·  还有多久下班':fontcolor=0xe4e4e7:fontsize=34:x=(w-text_w)/2:y=1530,
      drawtext=fontfile='$FONT':text='今年还要上多少天班  ·  日盼下班  ·  终盼退休':fontcolor=0xa1a1aa:fontsize=27:x=(w-text_w)/2:y=1600,
      fade=t=in:st=0:d=0.35:color=0x0a1211,fade=t=out:st=8.6:d=0.4:color=0x0a1211[dashboard];
    [2:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawgrid=w=90:h=90:t=1:c=0x38bdf8@0.04,
      drawtext=fontfile='$FONT':text='不想打开终端？':fontcolor=0xa1a1aa:fontsize=36:x=(w-text_w)/2:y=420,
      drawtext=fontfile='$FONT':text='网页版也能先算':fontcolor=0xe4e4e7:fontsize=76:x=(w-text_w)/2:y=560,
      drawtext=fontfile='$FONT':text='本地保存自己的':fontcolor=0x34d399:fontsize=42:x=(w-text_w)/2:y=760,
      drawtext=fontfile='$FONT':text='匿名查看大家的':fontcolor=0x38bdf8:fontsize=42:x=(w-text_w)/2:y=840,
      drawbox=x=100:y=1050:w=880:h=125:color=0x101c1a@0.95:t=fill,
      drawbox=x=100:y=1050:w=880:h=125:color=0x38bdf8@0.45:t=2,
      drawtext=fontfile='$FONT':text='wnma3mz.github.io/yuxin/':fontcolor=0x38bdf8:fontsize=34:x=(w-text_w)/2:y=1091,
      drawtext=fontfile='$FONT':text='无需登录  ·  不生成浏览器指纹':fontcolor=0xa1a1aa:fontsize=28:x=(w-text_w)/2:y=1320,
      fade=t=in:st=0:d=0.35:color=0x0a1211,fade=t=out:st=2.7:d=0.4:color=0x0a1211[web];
    [3:v]fps=30,settb=AVTB,setpts=PTS-STARTPTS,format=yuv420p,
      drawgrid=w=90:h=90:t=1:c=0x34d399@0.05,
      drawtext=fontfile='$FONT':text='YUXIN  ·  余薪':fontcolor=0x34d399:fontsize=92:x=(w-text_w)/2:y=410,
      drawtext=fontfile='$FONT':text='摸鱼有数，下班有期':fontcolor=0xe4e4e7:fontsize=54:x=(w-text_w)/2:y=570,
      drawtext=fontfile='$FONT':text='GitHub 搜索':fontcolor=0xa1a1aa:fontsize=34:x=(w-text_w)/2:y=850,
      drawbox=x=100:y=940:w=880:h=130:color=0x101c1a@0.95:t=fill,
      drawbox=x=100:y=940:w=880:h=130:color=0x34d399@0.5:t=2,
      drawtext=fontfile='$FONT':text='wnma3mz / yuxin':fontcolor=0xe4e4e7:fontsize=52:x=(w-text_w)/2:y=980,
      drawtext=fontfile='$FONT':text='brew install wnma3mz/tap/yuxin':fontcolor=0xa7f3d0:fontsize=29:x=(w-text_w)/2:y=1190,
      drawtext=fontfile='$FONT':text='wnma3mz.github.io/yuxin/':fontcolor=0x38bdf8:fontsize=31:x=(w-text_w)/2:y=1320,
      drawtext=fontfile='$FONT':text='开源  ·  本地运行  ·  macOS / Windows / Linux':fontcolor=0xd4d4d8:fontsize=27:x=(w-text_w)/2:y=1450,
      drawtext=fontfile='$FONT':text='▌':fontcolor=0x34d399:fontsize=42:x=(w-text_w)/2+315:y=1178:alpha='lt(mod(t,1),0.55)',
      fade=t=in:st=0:d=0.35:color=0x0a1211[outro];
    [intro][dashboard][web][outro]concat=n=4:v=1:a=0[out]
  " \
  -map "[out]" -c:v libx264 -preset slow -crf 18 -pix_fmt yuv420p \
  -movflags +faststart -an "$VISUAL"

"$FFMPEG" -hide_banner -loglevel error -y \
  -i "$VISUAL" -i "$VOICE_TRACK" \
  -map 0:v -map 1:a -c:v copy -c:a copy -movflags +faststart -shortest "$OUTPUT"

echo "已生成中文版竖屏推广视频：$OUTPUT"
