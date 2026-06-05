#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

image_tag="${1:-hydra:test}"
version="${2:-dev}"
context_dir="$(mktemp -d "${TMPDIR:-/tmp}/hydra-container.XXXXXX")"
amd64_binary="${repo_root}/dist/hydra_linux_amd64_v2/hydra"
arm64_binary=""

if [[ -f "${repo_root}/dist/hydra_linux_arm64/hydra" ]]; then
  arm64_binary="${repo_root}/dist/hydra_linux_arm64/hydra"
elif [[ -f "${repo_root}/dist/hydra_linux_arm64_v8.0/hydra" ]]; then
  arm64_binary="${repo_root}/dist/hydra_linux_arm64_v8.0/hydra"
fi

if [[ ! -f "${amd64_binary}" || -z "${arm64_binary}" ]]; then
  echo "Expected goreleaser binaries at dist/hydra_linux_amd64_v2/hydra and dist/hydra_linux_arm64/hydra (or dist/hydra_linux_arm64_v8.0/hydra)" >&2
  exit 1
fi

cleanup() {
  rm -rf "${context_dir}"
}
trap cleanup EXIT

mkdir -p "${context_dir}/linux/amd64" "${context_dir}/linux/arm64"
cp "${amd64_binary}" "${context_dir}/linux/amd64/hydra"
cp "${arm64_binary}" "${context_dir}/linux/arm64/hydra"

docker build \
  -f "${repo_root}/tools/build-container-image/Dockerfile" \
  --build-arg "VERSION=${version}" \
  -t "${image_tag}" \
  "${context_dir}"
