#!/bin/sh
# Osmium installer for Linux and macOS.
#
#   curl -fsSL https://osmiummc.com/install.sh | sh
#
# Environment:
#   OSMIUM_VERSION      a release tag to install, e.g. v0.1.1 (default: latest)
#   OSMIUM_INSTALL_DIR  where to put the binary (default: ~/.local/bin)

set -eu

REPO="limelamp/osmium"

if [ -t 1 ]; then
  BOLD="$(printf '\033[1m')"
  DIM="$(printf '\033[2m')"
  RED="$(printf '\033[31m')"
  RESET="$(printf '\033[0m')"
else
  BOLD="" DIM="" RED="" RESET=""
fi

info() { printf '%s\n' "$*"; }
note() { printf '%s%s%s\n' "$DIM" "$*" "$RESET"; }
fail() {
  printf '%serror:%s %s\n' "$RED" "$RESET" "$*" >&2
  exit 1
}

has() { command -v "$1" >/dev/null 2>&1; }

fetch() {
  if has curl; then
    curl -fsSL --retry 2 -o "$2" "$1"
  else
    wget -q -O "$2" "$1"
  fi
}

latest_tag() {
  if has curl; then
    url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")"
    tag="${url##*/}"
  else
    tag="$(wget -q -O - "https://api.github.com/repos/$REPO/releases/latest" |
      sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
  fi
  case "$tag" in
    v[0-9]*) printf '%s' "$tag" ;;
    *) fail "could not find the latest release of $REPO" ;;
  esac
}

sha256() {
  if has sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif has shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    fail "need sha256sum or shasum to verify the download"
  fi
}

main() {
  has curl || has wget || fail "need curl or wget"
  has tar || fail "need tar"

  case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    MINGW* | MSYS* | CYGWIN*)
      fail "on Windows, run this in PowerShell instead: irm https://osmiummc.com/install.ps1 | iex"
      ;;
    *) fail "unsupported OS: $(uname -s). Try: go install github.com/$REPO@latest" ;;
  esac

  case "$(uname -m)" in
    x86_64 | amd64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) fail "unsupported architecture: $(uname -m). Try: go install github.com/$REPO@latest" ;;
  esac

  # An arm64 Mac running this shell under Rosetta reports x86_64.
  if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ] &&
    [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]; then
    arch="arm64"
  fi

  tag="${OSMIUM_VERSION:-}"
  [ -n "$tag" ] || tag="$(latest_tag)"
  case "$tag" in v*) ;; *) tag="v$tag" ;; esac
  version="${tag#v}"

  dir="${OSMIUM_INSTALL_DIR:-$HOME/.local/bin}"
  archive="osmium_${version}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/$tag"

  info "Installing ${BOLD}osmium $version${RESET} ($os/$arch)"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  trap 'exit 130' INT TERM

  fetch "$base/$archive" "$tmp/$archive" || fail "could not download $base/$archive"
  fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt"

  want="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
  [ -n "$want" ] || fail "$archive is not listed in checksums.txt"
  [ "$(sha256 "$tmp/$archive")" = "$want" ] || fail "checksum mismatch for $archive"

  tar -xzf "$tmp/$archive" -C "$tmp" osmium

  mkdir -p "$dir"
  # Copy then rename, so a running osmium is replaced rather than overwritten.
  cp "$tmp/osmium" "$dir/.osmium.new"
  chmod 755 "$dir/.osmium.new"
  mv -f "$dir/.osmium.new" "$dir/osmium"

  info "Installed to $dir/osmium"
  info ""

  case ":$PATH:" in
    *":$dir:"*) ;;
    *)
      info "$dir is not on your PATH. Add it with:"
      case "${SHELL:-}" in
        */zsh) info "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.zshrc" ;;
        */fish) info "  fish_add_path $dir" ;;
        *) info "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.bashrc" ;;
      esac
      info "then open a new terminal."
      info ""
      ;;
  esac

  if ! has java; then
    note "No java found. A Minecraft server needs Java 21 for Minecraft 1.20.5 and newer:"
    note "https://osmiummc.com/docs/getting-started/installation"
    info ""
  fi

  info "Next:"
  info "  osmium completion install   # tab completion, once"
  info "  osmium                      # open the interface"
}

main "$@"
