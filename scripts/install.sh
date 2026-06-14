#!/usr/bin/env sh
set -eu

repo="${VFLOW_REPO:-nerveband/vflow}"
version="${VFLOW_VERSION:-latest}"
bin_dir="${VFLOW_BIN_DIR:-/usr/local/bin}"

echo "Install vflow from github.com/${repo} (${version})"
echo "Download the release archive and checksums.txt, verify SHA256, then install vflow into ${bin_dir}."
echo "This script is a template until the first tagged release exists."
