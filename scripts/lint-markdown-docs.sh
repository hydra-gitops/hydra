#!/usr/bin/env bash
set -euo pipefail

readonly script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly repo_root="$(cd "${script_dir}/.." && pwd)"
readonly tool_dir="${repo_root}/tools/markdownlint"
readonly config_path="${repo_root}/.markdownlint.json"
readonly markdownlint_bin="${tool_dir}/node_modules/.bin/markdownlint"

usage() {
  cat >&2 <<'EOF'
Usage: lint-markdown-docs.sh [--fix|-f] [markdownlint-args...]

  --fix, -f   Apply markdownlint-cli auto-fixes where supported (same as
              passing -f through to markdownlint-cli).

With no file/glob arguments, lints all Hydra docs under docs/**/*.md (relative
to the hydra/ directory).

Examples:
  ./scripts/lint-markdown-docs.sh
  ./scripts/lint-markdown-docs.sh --fix
  ./scripts/lint-markdown-docs.sh --fix docs/manual/README.md
EOF
}

if [[ ! -f "${config_path}" ]]; then
  echo "Missing markdownlint config: ${config_path}" >&2
  exit 1
fi

if [[ ! -x "${markdownlint_bin}" ]]; then
  echo "Missing local markdownlint installation: ${markdownlint_bin}" >&2
  echo "Run: cd \"${tool_dir}\" && npm ci" >&2
  exit 1
fi

cd "${repo_root}"

fix=0
pass_args=()
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -h | --help)
      usage
      exit 0
      ;;
    --fix | -f)
      fix=1
      shift
      ;;
    *)
      pass_args+=("$1")
      shift
      ;;
  esac
done

cmd=( "${markdownlint_bin}" --config "${config_path}" )
if [[ "${fix}" -eq 1 ]]; then
  cmd+=( -f )
fi

if [[ "${#pass_args[@]}" -gt 0 ]]; then
  cmd+=( "${pass_args[@]}" )
else
  cmd+=( "docs/**/*.md" )
fi

exec "${cmd[@]}"
