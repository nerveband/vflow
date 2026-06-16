#!/usr/bin/env sh
set -eu

repo="${VFLOW_REPO:-nerveband/vflow}"
version="${VFLOW_VERSION:-latest}"
bin_dir="${VFLOW_BIN_DIR:-$HOME/.local/bin}"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

log() {
  printf '[vflow-install] %s\n' "$1"
}

fail() {
  printf '[vflow-install] ERROR: %s\n' "$1" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

need curl
need tar
need shasum

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin) os="darwin" ;;
  linux) os="linux" ;;
  *) fail "Unsupported OS: $os" ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fail "Unsupported architecture: $arch" ;;
esac

if [ "$version" = "latest" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  [ -n "$version" ] || fail "Could not resolve latest release"
fi

archive="vflow_${version#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${version}"

log "Installing ${repo} ${version} for ${os}/${arch}"
curl -fsSL "${base_url}/${archive}" -o "${tmp_dir}/${archive}"
curl -fsSL "${base_url}/checksums.txt" -o "${tmp_dir}/checksums.txt"

(
  cd "$tmp_dir"
  grep "  ${archive}\$" checksums.txt | shasum -a 256 -c -
  tar -xzf "$archive"
)

[ -f "${tmp_dir}/vflow" ] || fail "Archive did not contain vflow binary"
chmod +x "${tmp_dir}/vflow"

mkdir -p "$bin_dir"
target="${bin_dir}/vflow"
if [ -e "$target" ]; then
  cp "$target" "${target}.bak"
fi
mv "${tmp_dir}/vflow" "$target"

log "Installed ${target}"
if command -v vflow >/dev/null 2>&1; then
  vflow version --format json || true
else
  log "Add ${bin_dir} to PATH to run vflow directly"
fi
