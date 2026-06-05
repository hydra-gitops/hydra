#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"

template_path="${script_dir}/templates/README.md.tpl"
out_path="${repo_root}/README.md"

repo_slug="${1:-hydra-gitops/hydra}"
image_ref="ghcr.io/${repo_slug,,}"

latest_release_badge="https://img.shields.io/github/v/release/${repo_slug}?sort=semver"
container_badge="https://img.shields.io/badge/container-ghcr.io-blue"

sed \
  -e "s|{{REPO}}|${repo_slug}|g" \
  -e "s|{{IMAGE}}|${image_ref}|g" \
  -e "s|{{LATEST_RELEASE_BADGE}}|${latest_release_badge}|g" \
  -e "s|{{CONTAINER_BADGE}}|${container_badge}|g" \
  "${template_path}" > "${out_path}"
