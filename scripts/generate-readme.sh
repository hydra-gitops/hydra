#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

repo_slug="${README_REPO_SLUG:-hydra-gitops/hydra}"
version="${1:-}"

if [[ -z "${version}" ]]; then
  latest_tag="$(git -C "${repo_root}" describe --tags --abbrev=0 2>/dev/null || true)"
  if [[ -n "${latest_tag}" ]]; then
    version="${latest_tag#v}"
  fi
fi

echo "[prepare] generate-readme.sh starting (repo=${repo_slug}, version=${version:-latest})"

go run "${script_dir}/render-readme-gotpl.go" \
  -template "${repo_root}/README.md.gotpl" \
  -output "${repo_root}/README.md" \
  -repo "${repo_slug}" \
  -version "${version}"

echo "[prepare] generate-readme.sh finished"
