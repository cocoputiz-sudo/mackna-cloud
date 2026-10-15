#!/usr/bin/env bash
# Pure verification of a cross-compiled MackNa bundle; never installs anything.
# Usage: bash scripts/verify-cross-arch-bundle.sh dist-amd64 amd64
set -Eeuo pipefail

if [[ $# != 2 ]]; then
  echo "Usage: $0 BUNDLE_DIST_DIRECTORY arm64|amd64" >&2
  exit 2
fi
DIST="$1"
ARCH="$2"
case "$ARCH" in
  arm64) MACHINE=183 ;;
  amd64) MACHINE=62 ;;
  *) echo "Unsupported architecture" >&2; exit 2 ;;
esac
AGENT="$DIST/mackna-agent-linux-$ARCH"
GATE="$DIST/mackna-gate-linux-$ARCH"
MANIFEST="$DIST/SHA256SUMS"
INFO="$DIST/BUILD_INFO.txt"

for tool in sha256sum grep sed od tr; do
  command -v "$tool" >/dev/null || { echo "Missing dependency: $tool" >&2; exit 2; }
done
for path in "$AGENT" "$GATE" "$MANIFEST" "$INFO"; do
  [[ -s "$path" ]] || { echo "Missing or empty bundle file: $path" >&2; exit 1; }
done

(cd "$DIST" && sha256sum -c SHA256SUMS) || exit 1
SOURCE_SHA="$(sed -n 's/^source_sha=//p' "$INFO")"
[[ "$SOURCE_SHA" =~ ^[a-f0-9]{40}$ ]] || { echo "Invalid source commit in manifest" >&2; exit 1; }

for exe in "$AGENT" "$GATE"; do
  MAGIC="$(od -An -tx1 -N4 "$exe" | tr -d ' \n')"
  FOUND_MACHINE="$(od -An -tu2 -j18 -N2 "$exe" | tr -d ' \n')"
  [[ "$MAGIC" == 7f454c46 && "$FOUND_MACHINE" == "$MACHINE" ]] || {
    echo "Wrong binary architecture: $exe" >&2
    exit 1
  }
done

for marker in "$SOURCE_SHA" "postgres_lab.js" "script-src 'self'; connect-src 'self'" "/api/v1/audit" "index-stats"; do
  grep -aqF "$marker" "$AGENT" || {
    echo "Missing functionality/build fingerprint from controller: $marker" >&2
    exit 1
  }
done
printf 'PASS: verified Linux %s build, SHA256, embedded UI/monitoring, source=%s\n' "$ARCH" "$SOURCE_SHA"
echo "No installation or service changes were made."
