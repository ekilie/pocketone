#!/bin/sh
# Installs PocketOne on Linux or macOS (https://pocketone.dev/docs/installation).
#
#   curl -fsSL https://pocketone.dev/install.sh | sh
#
# Linux: installs the release's .deb, .rpm or .apk package when the system has apt, dnf/yum/zypper or apk and you can
# be root (it uses sudo); the package adds a systemd service (https://pocketone.dev/docs/guides/linux-server). Otherwise, and on
# macOS, it puts the pocketone binary in /usr/local/bin. Run it again to upgrade.
#
# Settings (environment variables; with curl | sh, put them before sh: curl ... | POCKETONE_VERSION=v1.2.3 sh):
#   POCKETONE_VERSION       a release tag, such as v0.0.1-alpha.10 (default: the newest release, prereleases included)
#   POCKETONE_INSTALL_DIR   where the binary goes when no package is used (default /usr/local/bin)
#   POCKETONE_NO_PACKAGE=1  install the plain binary on Linux too, never a package
#   POCKETONE_DOWNLOAD_URL  the folder holding the release files (default: the GitHub release of POCKETONE_VERSION)
#
# The whole script is a function called on its last line, so a download cut short runs nothing.

set -eu

REPO=pocketonehq/pocketone

say() { printf '%s\n' "$*" >&2; }
fail() {
	say "install.sh: $*"
	exit 1
}
has() { command -v "$1" >/dev/null 2>&1; }

# as_root runs a command as root: directly when we are root, else through sudo (which may ask for your password)
as_root() {
	if [ "$(id -u)" = 0 ]; then
		"$@"
	elif has sudo; then
		sudo "$@"
	else
		fail "this step needs root and sudo isn't installed: $*"
	fi
}

can_be_root() { [ "$(id -u)" = 0 ] || has sudo; }

fetch() { # fetch <url> <file>
	curl -fsSL --retry 3 --connect-timeout 15 -o "$2" "$1"
}

sha256() {
	if has sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif has shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		fail "needs sha256sum or shasum to check the download"
	fi
}

newest_tag() {
	curl -fsSL --retry 3 --connect-timeout 15 "https://api.github.com/repos/$REPO/releases?per_page=1" |
		grep '"tag_name"' | head -n 1 | cut -d'"' -f4
}

main() {
	has curl || fail "needs curl"

	os=$(uname -s)
	case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "no PocketOne build for $os here; download one from https://github.com/$REPO/releases" ;;
	esac

	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	armv7l | armv7) arch=armv7 ;;
	ppc64le) arch=ppc64le ;;
	s390x) arch=s390x ;;
	*) fail "no PocketOne build for $os/$arch; see https://github.com/$REPO/releases" ;;
	esac
	if [ "$os" = darwin ] && [ "$arch" != amd64 ] && [ "$arch" != arm64 ]; then
		fail "no PocketOne build for macOS/$arch"
	fi

	tag=${POCKETONE_VERSION:-}
	if [ -z "$tag" ]; then
		tag=$(newest_tag) || true
		[ -n "$tag" ] || fail "couldn't look up the newest release on GitHub; set POCKETONE_VERSION=vX.Y.Z"
	fi
	version=${tag#v}
	base=${POCKETONE_DOWNLOAD_URL:-https://github.com/$REPO/releases/download/$tag}
	name="pocketone_${version}_${os}_${arch}"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "PocketOne $tag for $os/$arch"
	fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "couldn't download $base/checksums.txt"

	# Linux: a package, when the release has one for this system's package manager and we can be root
	pkg=""
	installer=""
	if [ "$os" = linux ] && [ "${POCKETONE_NO_PACKAGE:-}" != 1 ] && can_be_root; then
		if has apt-get && has dpkg; then
			pkg=deb installer=apt-get
		elif has dnf; then
			pkg=rpm installer=dnf
		elif has yum; then
			pkg=rpm installer=yum
		elif has zypper; then
			pkg=rpm installer=zypper
		elif has apk; then
			pkg=apk installer=apk
		fi
		if [ -n "$pkg" ] && ! grep -q " $name.$pkg\$" "$tmp/checksums.txt"; then
			say "This release has no .$pkg package; installing the binary instead."
			pkg=""
		fi
	fi

	if [ -n "$pkg" ]; then
		file="$name.$pkg"
	else
		file="$name.zip"
	fi
	say "Downloading $file"
	fetch "$base/$file" "$tmp/$file" || fail "couldn't download $base/$file"
	want=$(grep " $file\$" "$tmp/checksums.txt" | cut -d' ' -f1)
	[ -n "$want" ] || fail "$file isn't in checksums.txt"
	[ "$(sha256 "$tmp/$file")" = "$want" ] || fail "$file doesn't match its checksum; try again"

	if [ -n "$pkg" ]; then
		say "Installing the package with $installer"
		case "$installer" in
		apt-get) as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y "$tmp/$file" ;;
		dnf | yum) as_root "$installer" install -y "$tmp/$file" ;;
		zypper) as_root zypper --non-interactive install --allow-unsigned-rpm "$tmp/$file" ;;
		apk) as_root apk add --allow-untrusted "$tmp/$file" ;;
		esac
		bin=/usr/bin/pocketone
	else
		if has unzip; then
			unzip -o -q "$tmp/$file" pocketone -d "$tmp"
		elif has busybox; then
			(cd "$tmp" && busybox unzip -o -q "$file" pocketone)
		elif has python3; then
			python3 -c 'import sys,zipfile; zipfile.ZipFile(sys.argv[1]).extract("pocketone", sys.argv[2])' "$tmp/$file" "$tmp"
		else
			fail "needs unzip to unpack $file"
		fi
		chmod 755 "$tmp/pocketone"

		dir=${POCKETONE_INSTALL_DIR:-/usr/local/bin}
		say "Installing pocketone into $dir"
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			mv -f "$tmp/pocketone" "$dir/pocketone"
		elif [ ! -d "$dir" ] && mkdir -p "$dir" 2>/dev/null; then
			mv -f "$tmp/pocketone" "$dir/pocketone"
		elif can_be_root; then
			as_root mkdir -p "$dir"
			as_root mv -f "$tmp/pocketone" "$dir/pocketone"
		else
			fail "can't write to $dir; set POCKETONE_INSTALL_DIR to a folder you own, such as \$HOME/.local/bin"
		fi
		bin="$dir/pocketone"
	fi

	"$bin" --version >&2 || fail "$bin didn't run"

	found=$(command -v pocketone 2>/dev/null || true)
	if [ -z "$found" ]; then
		say "Note: $(dirname "$bin") isn't on your PATH; add it, or run $bin."
	elif [ "$found" != "$bin" ]; then
		say "Note: another pocketone, $found, comes first on your PATH."
	fi

	say ""
	if [ -n "$pkg" ]; then
		say "Done. To run PocketOne as a service, see https://pocketone.dev/docs/guides/linux-server"
	else
		say "Done. Start an app with: pocketone serve"
		say "Upgrade later with: pocketone upgrade (or run this script again)"
	fi
}

main "$@"
