#!/usr/bin/env bash

set -euo pipefail

log_sign_tag() {
  printf '[sign-release-tag.sh] %s\n' "$*" >&2
}

release_tag="${1:-}"
release_head="${2:-}"

if [[ -z "${release_tag}" ]]; then
  echo "Usage: $0 <tag> [commit-sha]" >&2
  exit 1
fi

if [[ -z "${release_head}" ]]; then
  release_head="$(git rev-list -n1 "${release_tag}" 2>/dev/null || true)"
fi

if [[ -z "${release_head}" ]]; then
  echo "Could not resolve commit for tag ${release_tag}" >&2
  exit 1
fi

log_sign_tag "signing tag ${release_tag} at ${release_head}"
git tag -s -f -m "${release_tag}" "${release_tag}" "${release_head}"
git tag -v "${release_tag}" >/dev/null
log_sign_tag "signed and verified tag ${release_tag}"

log_sign_tag "force pushing signed tag ${release_tag} to origin"
git push origin "+refs/tags/${release_tag}:refs/tags/${release_tag}"
log_sign_tag "force pushed signed tag ${release_tag} to origin"
