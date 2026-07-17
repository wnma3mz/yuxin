#!/bin/sh

set -eu

if [ "$#" -ne 4 ]; then
  echo "用法：$0 VERSION ARM64_SHA256 X86_64_SHA256 OUTPUT" >&2
  exit 2
fi

version="$1"
arm64_sha256="$2"
x86_64_sha256="$3"
output="$4"

case "$version" in
  ''|*[!0-9.]*|.*|*..*|*.)
    echo "版本号必须只包含数字和点，且不能有空段。" >&2
    exit 2
    ;;
esac

validate_checksum() {
  checksum="$1"
  architecture="$2"
  if [ "${#checksum}" -ne 64 ]; then
    echo "$architecture SHA-256 必须是 64 位十六进制字符串。" >&2
    exit 2
  fi
  case "$checksum" in
    *[!0-9a-f]*)
      echo "$architecture SHA-256 必须是小写十六进制字符串。" >&2
      exit 2
      ;;
  esac
}

validate_checksum "$arm64_sha256" "arm64"
validate_checksum "$x86_64_sha256" "x86_64"
mkdir -p "$(dirname "$output")"

cat > "$output" <<EOF
class Yuxin < Formula
  desc "Terminal dashboard for salary, savings, and retirement progress"
  homepage "https://github.com/wnma3mz/yuxin"
  version "$version"
  license "MIT"

  depends_on :macos

  if Hardware::CPU.arm?
    url "https://github.com/wnma3mz/yuxin/releases/download/v#{version}/yuxin-macos-arm64.zip"
    sha256 "$arm64_sha256"
  else
    url "https://github.com/wnma3mz/yuxin/releases/download/v#{version}/yuxin-macos-x86_64.zip"
    sha256 "$x86_64_sha256"
  end

  def install
    bin.install "yuxin"
  end

  test do
    assert_match "余薪 Yuxin #{version}", shell_output("#{bin}/yuxin --version")
  end
end
EOF
