#!/usr/bin/env bash
# Build the Hydra user manual static site (output: hydra/docs/site/site/).
set -euo pipefail

readonly site_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${site_dir}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

python3 -m venv .venv
# shellcheck source=/dev/null
source .venv/bin/activate
pip install -q --upgrade pip
pip install -q -r requirements.txt
pip install -q -e .

mkdocs build "$@"

echo "Built site: ${site_dir}/site/"
echo "See README.md for local preview commands."
