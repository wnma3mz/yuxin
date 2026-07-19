#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT=${1:-$ROOT/docs/assets/yuxin-social-voice.m4a}
PYTHON=${YUXIN_SOCIAL_TTS_PYTHON:-python3}
VOICE=${YUXIN_SOCIAL_TTS_VOICE:-zh-CN-XiaoxiaoNeural}
RATE=${YUXIN_SOCIAL_TTS_RATE:-+12%}
PITCH=${YUXIN_SOCIAL_TTS_PITCH:--2Hz}
FFMPEG=${YUXIN_FFMPEG:-ffmpeg}

command -v "$PYTHON" >/dev/null 2>&1 || { echo "需要 Python 3" >&2; exit 1; }
command -v "$FFMPEG" >/dev/null 2>&1 || { echo "需要 ffmpeg" >&2; exit 1; }
"$PYTHON" -c 'from importlib.metadata import version; raise SystemExit(tuple(map(int, version("edge-tts").split("."))) < (7, 2, 8))' >/dev/null 2>&1 || {
  echo "需要 edge-tts 7.2.8 或更新版本；推荐运行：python3 -m pip install --upgrade edge-tts" >&2
  exit 1
}

mkdir -p "$(dirname -- "$OUTPUT")"
TEMPORARY=$(mktemp -d "${TMPDIR:-/tmp}/yuxin-social-voice.XXXXXX")
trap 'rm -rf "$TEMPORARY"' EXIT HUP INT TERM

speak() {
  name=$1
  text=$2
  "$PYTHON" -m edge_tts \
    --voice "$VOICE" \
    --rate="$RATE" \
    --pitch="$PITCH" \
    --text "$text" \
    --write-media "$TEMPORARY/$name.mp3"
}

speak 01 '打开余薪网页版，先看看今天赚了多少。'
speak 02 '工资会跟着时间往前跳。距离下班、下一个盼头，还有现在躺平能花多少，都在这一屏。'
speak 03 '再看看匿名样本，听听打工人的匿名回声。这里只展示聚合结果。'
speak 04 '余薪网页版。摸鱼有数，下班有期。'

"$FFMPEG" -hide_banner -loglevel error -y \
  -i "$TEMPORARY/01.mp3" \
  -i "$TEMPORARY/02.mp3" \
  -i "$TEMPORARY/03.mp3" \
  -i "$TEMPORARY/04.mp3" \
  -filter_complex "
    [0:a]aresample=48000,afade=t=out:st=3.72:d=0.12,apad,atrim=0:4.0[a0];
    [1:a]aresample=48000,afade=t=out:st=7.58:d=0.12,apad,atrim=0:7.8[a1];
    [2:a]aresample=48000,afade=t=out:st=6.04:d=0.12,apad,atrim=0:6.3[a2];
    [3:a]aresample=48000,afade=t=out:st=3.78:d=0.10,apad,atrim=0:3.9[a3];
    [a0][a1][a2][a3]concat=n=4:v=0:a=1,alimiter=limit=0.95[out]
  " \
  -map "[out]" -c:a aac -b:a 160k -ar 48000 -movflags +faststart "$OUTPUT"

echo "已生成竖屏自然神经女声音轨：$OUTPUT"
