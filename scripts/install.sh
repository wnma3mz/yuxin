#!/bin/sh

set -eu

repository="https://github.com/wnma3mz/yuxin"

case "$(uname -s)" in
  Darwin) system="macos" ;;
  Linux) system="linux" ;;
  *) echo "不支持当前操作系统。" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) architecture="arm64" ;;
  x86_64|amd64) architecture="x86_64" ;;
  *) echo "不支持当前处理器架构。" >&2; exit 1 ;;
esac

if [ "$system" = "linux" ] && [ "$architecture" = "arm64" ]; then
  echo "当前发布版尚不支持 Linux ARM64。" >&2
  exit 1
fi

for command in curl unzip; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "缺少必需命令：$command" >&2
    exit 1
  fi
done

asset="yuxin-${system}-${architecture}.zip"
base="${YUXIN_RELEASE_BASE:-${repository}/releases/latest/download}"
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT HUP INT TERM

printf '正在下载 Yuxin 最新正式版 (%s/%s)...\n' "$system" "$architecture"
if ! curl --fail --location --silent --show-error "${base}/${asset}" --output "${temporary}/${asset}"; then
  echo "下载失败：当前还没有可用于此平台的正式版。" >&2
  exit 1
fi
curl --fail --location --silent --show-error "${base}/${asset}.sha256" --output "${temporary}/${asset}.sha256"

expected="$(awk 'NR == 1 { print $1 }' "${temporary}/${asset}.sha256")"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temporary}/${asset}" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "${temporary}/${asset}" | awk '{ print $1 }')"
fi
if [ -z "$expected" ] || [ "$actual" != "$expected" ]; then
  echo "SHA-256 校验失败。" >&2
  exit 1
fi

unzip -q "${temporary}/${asset}" -d "${temporary}/release"
executable="$(find "${temporary}/release" -type f -name yuxin -print -quit)"
if [ -z "$executable" ]; then
  echo "发布包中缺少 yuxin。" >&2
  exit 1
fi

install_directory="${YUXIN_INSTALL_DIR:-${HOME}/.local/bin}"
mkdir -p "$install_directory"
cp "$executable" "${install_directory}/yuxin"
chmod 755 "${install_directory}/yuxin"

printf '已安装到 %s/yuxin\n' "$install_directory"
case ":$PATH:" in
  *":$install_directory:"*) ;;
  *) printf '请将 %s 加入 PATH。\n' "$install_directory" ;;
esac
