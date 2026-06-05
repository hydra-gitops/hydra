#!/usr/bin/env bash

set -euo pipefail

: "${SOPS_AGE_KEY_RELEASE:=${SOPS_AGE_KEY:-}}"
if [[ -z "${SOPS_AGE_KEY_RELEASE}" ]]; then
  SOPS_AGE_KEY_RELEASE="$(sops --decrypt --extract '["age_keys"]["release"]["private_key"]' .github/secrets/age-pipeline-keys.sops.yaml 2>/dev/null | grep AGE-SECRET-KEY-)"
fi
if [[ -z "${SOPS_AGE_KEY_RELEASE}" ]]; then
  echo "SOPS_AGE_KEY_RELEASE or SOPS_AGE_KEY must be set" >&2
  exit 1
fi

export SOPS_AGE_KEY="${SOPS_AGE_KEY_RELEASE}"

tmp_dir="${RUNNER_TEMP:-/tmp}"
mkdir -p "${tmp_dir}"
git_signing_key="$(mktemp "${tmp_dir}/hydra_git_signing.XXXXXX.key")"
allowed_signers_file="$(mktemp "${tmp_dir}/hydra_allowed_signers.XXXXXX")"
git_signing_key_raw="$(sops --decrypt --extract '["git_signing"]["private_key"]' .github/secrets/release.sops.yaml)"
if [[ "${git_signing_key_raw}" == *\\n* ]]; then
  printf '%b\n' "${git_signing_key_raw}" > "${git_signing_key}"
else
  printf '%s\n' "${git_signing_key_raw}" > "${git_signing_key}"
fi
chmod 600 "${git_signing_key}"

cleanup() {
  rm -f "${git_signing_key}" "${git_signing_key}.pub" "${allowed_signers_file}"
  if [[ -n "${git_wrapper_dir:-}" ]]; then
    rm -rf "${git_wrapper_dir}"
  fi
}
trap cleanup EXIT

if ! ssh-keygen -y -P "" -f "${git_signing_key}" >/dev/null 2>&1; then
  echo "Extracted git_signing.private_key is invalid or requires a passphrase; CI needs an unencrypted SSH signing key" >&2
  exit 1
fi

git_user_name="${SEMANTIC_RELEASE_GIT_USER_NAME:-${GIT_AUTHOR_NAME:-Hydra Release}}"
git_user_email="${SEMANTIC_RELEASE_GIT_USER_EMAIL:-${GIT_AUTHOR_EMAIL:-release@hydra-gitops.org}}"

git_signing_pub="$(ssh-keygen -y -P "" -f "${git_signing_key}")"
printf '%s %s\n' "${git_user_email}" "${git_signing_pub}" > "${allowed_signers_file}"

export GIT_AUTHOR_NAME="${git_user_name}"
export GIT_AUTHOR_EMAIL="${git_user_email}"
export GIT_COMMITTER_NAME="${git_user_name}"
export GIT_COMMITTER_EMAIL="${git_user_email}"

# Inject signing config for all git invocations in this process tree without
# touching repository-local git config.
export GIT_CONFIG_COUNT=5
export GIT_CONFIG_KEY_0="gpg.format"
export GIT_CONFIG_VALUE_0="ssh"
export GIT_CONFIG_KEY_1="user.signingkey"
export GIT_CONFIG_VALUE_1="${git_signing_key}"
export GIT_CONFIG_KEY_2="commit.gpgsign"
export GIT_CONFIG_VALUE_2="true"
export GIT_CONFIG_KEY_3="tag.gpgSign"
export GIT_CONFIG_VALUE_3="false"
export GIT_CONFIG_KEY_4="gpg.ssh.allowedSignersFile"
export GIT_CONFIG_VALUE_4="${allowed_signers_file}"
export GIT_TERMINAL_PROMPT="0"
export GIT_EDITOR=:

log_release() {
  printf '[semantic-release.sh] %s\n' "$*" >&2
}

real_git_bin="$(command -v git)"
git_wrapper_dir="$(mktemp -d "${tmp_dir}/hydra_git_wrapper.XXXXXX")"
git_bin="${git_wrapper_dir}/git-bin"
cat > "${git_bin}" <<EOF
#!/usr/bin/env bash
set -euo pipefail

printf '[git-bin] cwd=%s cmd=%q' "\$(pwd)" "${real_git_bin}" >&2
for arg in "\$@"; do
  printf ' %q' "\${arg}" >&2
done
printf '\n' >&2

exec "${real_git_bin}" "\$@"
EOF
chmod 700 "${git_bin}"

cat > "${git_wrapper_dir}/git" <<EOF
#!/usr/bin/env bash
set -euo pipefail

log_git_wrapper() {
  printf '[git-wrapper] cwd=%s cmd=git' "\$(pwd)" >&2
  for arg in "\$@"; do
    printf ' %q' "\${arg}" >&2
  done
  printf '\n' >&2
}

log_git_wrapper "\$@"
exec "${git_bin}" "\$@"
EOF
chmod 700 "${git_wrapper_dir}/git"
export PATH="${git_wrapper_dir}:${PATH}"
export DEBUG="*"
log_release "starting semantic-release"
npx -y \
  -p semantic-release@25.0.3 \
  -p conventional-changelog-conventionalcommits \
  -p @semantic-release/changelog \
  -p @semantic-release/exec \
  -p @semantic-release/git \
  semantic-release --debug

log_release "semantic-release finished, continuing publish pipeline"
