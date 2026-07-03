#!/usr/bin/env bash
# Install the brain CLI from GitHub Releases (macOS / Linux). No Go toolchain needed.
#
#   curl -fsSL https://raw.githubusercontent.com/muthuishere/brain/main/install.sh | bash
#   # or, from a clone:
#   ./install.sh
#
# Env:
#   BRAIN_VERSION  release tag to install (default: latest)
#   BRAIN_BINDIR   where to put the binary (default: ~/.local/bin)
set -euo pipefail

repo="muthuishere/brain"
version="${BRAIN_VERSION:-latest}"
bindir="${BRAIN_BINDIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"   # darwin | linux
case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

asset="brain_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  url="https://github.com/${repo}/releases/latest/download/${asset}"
else
  url="https://github.com/${repo}/releases/download/${version}/${asset}"
fi

echo "downloading ${asset} (${version})…"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/brain.tar.gz"
tar -xzf "$tmp/brain.tar.gz" -C "$tmp"

mkdir -p "$bindir"
install -m 0755 "$tmp/brain" "$bindir/brain"
echo "installed $bindir/brain"

"$bindir/brain" install-skills

case ":$PATH:" in
  *":$bindir:"*) : ;;
  *) echo "note: add $bindir to your PATH" ;;
esac

echo "done. try:  brain --repo /tmp/mybrain init"
