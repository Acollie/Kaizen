#!/usr/bin/env bash
# Render the Homebrew formula for a Kaizen release.
#
# Usage:
#   scripts/update-homebrew-formula.sh <tag> <output-path>
#
# Example (writing into a local checkout of the tap repo):
#   scripts/update-homebrew-formula.sh v1.2.0 ../homebrew-tap/Formula/kaizen.rb
#
# Downloads the GitHub tag tarball for <tag>, computes its sha256, and writes
# packaging/homebrew/kaizen.rb with the url and sha256 filled in to
# <output-path>. Running it again with the same tag yields an identical file.
#
# Environment overrides:
#   KAIZEN_REPO         GitHub owner/repo (default: Acollie/Kaizen)
#   KAIZEN_TARBALL_URL  URL to download instead of the GitHub tag tarball
#                       (for local testing, e.g. file:///tmp/kaizen.tar.gz).
#                       The formula's url still points at that URL.
set -euo pipefail

usage() {
  echo "Usage: $0 <tag> <output-path>" >&2
  exit 64
}

[[ $# -eq 2 ]] || usage

tag="$1"
output_path="$2"

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "error: tag must look like v1.2.3 (got '$tag')" >&2
  exit 64
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
template="${script_dir}/../packaging/homebrew/kaizen.rb"
repo="${KAIZEN_REPO:-Acollie/Kaizen}"
tarball_url="${KAIZEN_TARBALL_URL:-https://github.com/${repo}/archive/refs/tags/${tag}.tar.gz}"

[[ -f "$template" ]] || { echo "error: template not found: $template" >&2; exit 1; }

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

echo "Downloading ${tarball_url}" >&2
curl --fail --silent --show-error --location --retry 3 \
  --output "${work_dir}/source.tar.gz" "$tarball_url"

if command -v sha256sum >/dev/null 2>&1; then
  sha256="$(sha256sum "${work_dir}/source.tar.gz" | awk '{print $1}')"
else
  sha256="$(shasum -a 256 "${work_dir}/source.tar.gz" | awk '{print $1}')"
fi

# Replace the url and sha256 lines of the template. Matching whole lines keeps
# this idempotent no matter what values the template currently holds.
awk -v url="$tarball_url" -v sha="$sha256" '
  /^  url "/    { print "  url \"" url "\""; next }
  /^  sha256 "/ { print "  sha256 \"" sha "\""; next }
  { print }
' "$template" > "${work_dir}/kaizen.rb"

grep -q "^  sha256 \"${sha256}\"$" "${work_dir}/kaizen.rb" || {
  echo "error: failed to substitute sha256 into formula" >&2
  exit 1
}

mkdir -p "$(dirname "$output_path")"
cp "${work_dir}/kaizen.rb" "$output_path"
echo "Wrote ${output_path} (${tag}, sha256 ${sha256})" >&2
