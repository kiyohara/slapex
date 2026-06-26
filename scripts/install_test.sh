#!/bin/sh
# Tests for scripts/install.sh.
#
# - OS/arch detection and asset-name mapping: stub `uname` on PATH and run the
#   installer in --dry-run mode (a fixed --version keeps it offline).
# - checksum verification: source install.sh and exercise verify_checksum.
#
# Run: sh scripts/install_test.sh

set -eu

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
installer="$here/install.sh"

# Load install.sh functions (verify_checksum, sha256_of) without running main.
# install.sh is linted separately, so don't follow the source here.
# shellcheck disable=SC1090,SC1091
SLAPEX_INSTALL_SH_NO_MAIN=1 . "$installer"

stub_dir=$(mktemp -d 2>/dev/null || mktemp -d -t slapex-test)
trap 'rm -rf "$stub_dir"' EXIT INT TERM

failures=0

# make_uname <kernel> <machine>: write a fake `uname` that reports them.
make_uname() {
	cat >"$stub_dir/uname" <<EOF
#!/bin/sh
if [ "\$1" = "-m" ]; then echo "$2"; else echo "$1"; fi
EOF
	chmod +x "$stub_dir/uname"
}

run_installer() {
	PATH="$stub_dir:$PATH" sh "$installer" --version v1.0.1 --dry-run
}

# expect_asset <kernel> <machine> <expected-asset>
expect_asset() {
	make_uname "$1" "$2"
	if out=$(run_installer 2>/dev/null) && printf '%s\n' "$out" | grep -q "^asset=$3\$"; then
		printf 'ok: %s/%s -> %s\n' "$1" "$2" "$3"
	else
		printf 'FAIL: %s/%s expected asset=%s, got:\n%s\n' "$1" "$2" "$3" "${out:-<none>}" >&2
		failures=$((failures + 1))
	fi
}

# expect_unsupported <kernel> <machine>: the installer must exit non-zero.
expect_unsupported() {
	make_uname "$1" "$2"
	if run_installer >/dev/null 2>&1; then
		printf 'FAIL: %s/%s should be unsupported but succeeded\n' "$1" "$2" >&2
		failures=$((failures + 1))
	else
		printf 'ok: %s/%s rejected\n' "$1" "$2"
	fi
}

# verify_checksum must accept a matching digest and reject a mismatch.
test_checksum() {
	work=$(mktemp -d 2>/dev/null || mktemp -d -t slapex-sum)
	bin="$work/slapex_linux_amd64"
	printf 'fake-binary-content\n' >"$bin"
	realhash=$(sha256_of "$bin")
	printf '%s  slapex_linux_amd64\n' "$realhash" >"$work/good.txt"
	printf '%s  slapex_linux_amd64\n' \
		"0000000000000000000000000000000000000000000000000000000000000000" >"$work/bad.txt"

	# die() calls exit, so run verify_checksum in a subshell to contain it.
	if (verify_checksum "$bin" "$work/good.txt" slapex_linux_amd64) 2>/dev/null; then
		printf 'ok: checksum match accepted\n'
	else
		printf 'FAIL: matching checksum was rejected\n' >&2
		failures=$((failures + 1))
	fi

	if (verify_checksum "$bin" "$work/bad.txt" slapex_linux_amd64) 2>/dev/null; then
		printf 'FAIL: mismatching checksum was accepted\n' >&2
		failures=$((failures + 1))
	else
		printf 'ok: checksum mismatch rejected\n'
	fi

	rm -rf "$work"
}

expect_asset Darwin arm64 slapex_darwin_arm64
expect_asset Darwin x86_64 slapex_darwin_amd64
expect_asset Linux x86_64 slapex_linux_amd64
expect_asset Linux amd64 slapex_linux_amd64
expect_asset Linux aarch64 slapex_linux_arm64
expect_asset Linux arm64 slapex_linux_arm64

expect_unsupported Linux armv7l
expect_unsupported FreeBSD amd64
expect_unsupported Windows_NT x86_64

test_checksum

if [ "$failures" -ne 0 ]; then
	printf '\n%d test(s) failed\n' "$failures" >&2
	exit 1
fi
printf '\nall install.sh tests passed\n'
