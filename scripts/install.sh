#!/bin/sh
# slapex installer
#
# Downloads the slapex binary from GitHub Releases, verifies its sha256
# checksum, and installs it into a bin directory.
#
# Quick install (latest release into /usr/local/bin):
#   curl -fsSL https://raw.githubusercontent.com/kiyohara/slapex/main/scripts/install.sh | sh
#
# Pass options through a pipe with `-s --`:
#   curl -fsSL .../install.sh | sh -s -- --bin-dir "$HOME/.local/bin"
#
# Or download first, then run:
#   curl -fsSLO https://raw.githubusercontent.com/kiyohara/slapex/main/scripts/install.sh
#   sh install.sh --version v1.0.1
#
# Options:
#   --version <tag>   Release tag to install (e.g. v1.0.1). Default: latest release.
#   --bin-dir <dir>   Install directory. Default: /usr/local/bin.
#   --dry-run         Print what would be installed (target, version, URLs) and exit.
#   --help            Show this help and exit.
#
# Environment variables (a matching flag takes precedence):
#   SLAPEX_VERSION    Same as --version.
#   SLAPEX_BIN_DIR    Same as --bin-dir.

set -eu

REPO="kiyohara/slapex"
DEFAULT_BIN_DIR="/usr/local/bin"

# Diagnostics go to stderr; stdout is reserved for the final install path.
log() { printf '%s\n' "$*" >&2; }
err() { printf 'error: %s\n' "$*" >&2; }
die() {
	err "$*"
	exit 1
}

usage() {
	cat >&2 <<'EOF'
slapex installer

Usage:
  curl -fsSL https://raw.githubusercontent.com/kiyohara/slapex/main/scripts/install.sh | sh
  sh install.sh [--version <tag>] [--bin-dir <dir>] [--dry-run]

Options:
  --version <tag>   Release tag to install (e.g. v1.0.1). Default: latest release.
  --bin-dir <dir>   Install directory. Default: /usr/local/bin.
  --dry-run         Print what would be installed and exit without changes.
  --help            Show this help and exit.

Environment variables (a matching flag takes precedence):
  SLAPEX_VERSION    Same as --version.
  SLAPEX_BIN_DIR    Same as --bin-dir.
EOF
}

have() { command -v "$1" >/dev/null 2>&1; }

# download <url> <dest>
download() {
	if have curl; then
		curl -fsSL "$1" -o "$2"
	elif have wget; then
		wget -qO "$2" "$1"
	else
		die "need curl or wget to download files"
	fi
}

# fetch_stdout <url> -> prints the response body to stdout
fetch_stdout() {
	if have curl; then
		curl -fsSL "$1"
	elif have wget; then
		wget -qO- "$1"
	else
		die "need curl or wget to download files"
	fi
}

detect_os() {
	kernel=$(uname -s)
	case "$kernel" in
	Darwin) echo darwin ;;
	Linux) echo linux ;;
	*) die "unsupported OS: $kernel (slapex supports macOS and Linux)" ;;
	esac
}

detect_arch() {
	machine=$(uname -m)
	case "$machine" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*) die "unsupported architecture: $machine (slapex supports amd64 and arm64)" ;;
	esac
}

# Resolve the latest release tag via the GitHub API (no JSON parser needed).
latest_version() {
	tag=$(fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" |
		grep '"tag_name"' | head -n1 | cut -d'"' -f4)
	[ -n "$tag" ] || die "could not resolve the latest release tag; pass --version <tag>"
	printf '%s\n' "$tag"
}

# sha256_of <file> -> prints the hex digest
sha256_of() {
	if have sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		die "need sha256sum or shasum to verify the checksum"
	fi
}

# verify_checksum <binary-file> <checksums-file> <asset-name>
verify_checksum() {
	expected=$(grep " $3\$" "$2" | cut -d' ' -f1)
	[ -n "$expected" ] || die "no checksum entry for $3 in the checksums file"
	actual=$(sha256_of "$1")
	if [ "$expected" != "$actual" ]; then
		die "checksum mismatch for $3 (expected $expected, got $actual)"
	fi
}

main() {
	version=${SLAPEX_VERSION:-}
	bin_dir=${SLAPEX_BIN_DIR:-$DEFAULT_BIN_DIR}
	dry_run=""

	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			shift
			[ $# -gt 0 ] || die "--version needs a value"
			version=$1
			;;
		--version=*) version=${1#--version=} ;;
		--bin-dir)
			shift
			[ $# -gt 0 ] || die "--bin-dir needs a value"
			bin_dir=$1
			;;
		--bin-dir=*) bin_dir=${1#--bin-dir=} ;;
		--dry-run) dry_run=1 ;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			err "unknown option: $1"
			usage
			exit 2
			;;
		esac
		shift
	done

	os=$(detect_os)
	arch=$(detect_arch)
	asset="slapex_${os}_${arch}"

	if [ -z "$version" ]; then
		log "Resolving the latest slapex release..."
		version=$(latest_version)
	fi

	base="https://github.com/$REPO/releases/download/$version"

	if [ -n "$dry_run" ]; then
		printf 'os=%s\n' "$os"
		printf 'arch=%s\n' "$arch"
		printf 'asset=%s\n' "$asset"
		printf 'version=%s\n' "$version"
		printf 'url=%s\n' "$base/$asset"
		printf 'checksums_url=%s\n' "$base/slapex_checksums.txt"
		printf 'bin_dir=%s\n' "$bin_dir"
		exit 0
	fi

	log "Installing slapex $version ($os/$arch) into $bin_dir"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t slapex)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	log "Downloading $asset..."
	download "$base/$asset" "$tmp/$asset" ||
		die "failed to download $base/$asset (check the version tag and your OS/arch)"

	log "Downloading checksums..."
	download "$base/slapex_checksums.txt" "$tmp/slapex_checksums.txt" ||
		die "failed to download the checksums file"

	log "Verifying checksum..."
	verify_checksum "$tmp/$asset" "$tmp/slapex_checksums.txt" "$asset"

	chmod +x "$tmp/$asset"

	dest="$bin_dir/slapex"
	# Use a plain mv only if we can actually write into bin_dir. mkdir -p on an
	# existing but non-writable dir (e.g. root-owned /usr/local/bin) returns 0,
	# so re-check writability after it to fall through to the sudo branch.
	if mkdir -p "$bin_dir" 2>/dev/null && [ -w "$bin_dir" ]; then
		mv "$tmp/$asset" "$dest"
	elif have sudo; then
		log "$bin_dir is not writable; using sudo (you may be prompted for your password)."
		if ! { sudo mkdir -p "$bin_dir" && sudo mv "$tmp/$asset" "$dest"; }; then
			die "sudo install failed; re-run with a writable --bin-dir"
		fi
	else
		die "cannot write to $bin_dir and sudo is not available.
Re-run with a writable directory, e.g.:
  sh install.sh --bin-dir \"\$HOME/.local/bin\"
then ensure that directory is on your PATH."
	fi

	log "Installed: $dest"
	if ! command -v slapex >/dev/null 2>&1; then
		log "note: $bin_dir is not on your PATH. Add it, for example:"
		log "  export PATH=\"$bin_dir:\$PATH\""
	fi

	# stdout: the final install path (handy for scripting).
	printf '%s\n' "$dest"
}

# Run main only when executed directly. Tests source this file with
# SLAPEX_INSTALL_SH_NO_MAIN=1 to load the functions without running it.
if [ "${SLAPEX_INSTALL_SH_NO_MAIN:-}" != "1" ]; then
	main "$@"
fi
