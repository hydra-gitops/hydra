# Hydra

[![Latest release](https://img.shields.io/github/v/release/hydra-gitops/hydra?sort=semver)](https://github.com/hydra-gitops/hydra/releases/tag/v1.0.1)
[![release](https://github.com/hydra-gitops/hydra/actions/workflows/release.yml/badge.svg)](https://github.com/hydra-gitops/hydra/actions/workflows/release.yml)
![Container image](https://img.shields.io/badge/container-ghcr.io-blue)

Hydra provides a standardized GitOps workflow for Helm and Argo CD with a CLI-first toolchain and reproducible release pipelines.

Latest signed release: [v1.0.1](https://github.com/hydra-gitops/hydra/releases/tag/v1.0.1)

## Install

### Homebrew (macOS and Linux)

```bash
brew install --cask hydra-gitops/tap/hydra
```

### Docker (linux/amd64 and linux/arm64)

```bash
docker pull ghcr.io/hydra-gitops/hydra:latest
docker run --rm ghcr.io/hydra-gitops/hydra:latest --help
```

### Download CLI archives

Release assets are published on each signed version tag:

- https://github.com/hydra-gitops/hydra/releases/tag/v1.0.1

Verify downloaded archives with the published checksum file:

```bash
curl -LO https://github.com/hydra-gitops/hydra/releases/download/v1.0.1/checksums.txt
shasum -a 256 --check checksums.txt
```

## Verify releases

Public keys are published in [.github/secrets/public-keys.yaml](.github/secrets/public-keys.yaml).

- Release tags are created as lightweight tags first and are rewritten to signed annotated tags immediately before push so `semantic-release` can keep its git-note metadata on the tagged commit.
- Release tag signatures are verified before release jobs start.
- Downloaded CLI archives can be checked against `checksums.txt`.
- CLI archives are signed during the release workflow.
- Published container images are signed by digest.

## CI tool installation

GitHub Actions installs `cosign`, `sops`, and `goreleaser` into `$HOME/.cosign`.
The `sigstore/cosign-installer` action adds that directory to `GITHUB_PATH`, so later
workflow steps can call these binaries directly without `sudo` or `/usr/local/bin`.

## Build locally

```bash
./scripts/build-container-image.sh hydra:test v0.0.0-local
```

Build release archives from the repo root with:

```bash
(
  cd hydra-go
  goreleaser release --clean --snapshot --config .goreleaser.yml
)
```

## Developer scripts

- Local container build: [scripts/build-container-image.sh](scripts/build-container-image.sh)
- Markdown linting: [scripts/lint-markdown-docs.sh](scripts/lint-markdown-docs.sh)
- Root README generation: [scripts/generate-readme.sh](scripts/generate-readme.sh)

## Documentation

- User and manual docs: [docs/](docs/)
- Go release config: [hydra-go/.goreleaser.yml](hydra-go/.goreleaser.yml)
- Release changelog: [CHANGELOG.md](CHANGELOG.md)
- Release process and platform matrix: [RELEASE.md](RELEASE.md)
- Renovate configuration: [renovate.json](renovate.json)
