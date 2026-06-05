# Develop Guide

This document describes the Hydra release process and the semantic-release tag and commit conventions.

## Goal

- Releases are generated automatically from `main`.
- Versions follow SemVer and are tagged as `vX.Y.Z`.
- Release tags are signed and verified before publish.

## Relevant Pipelines

- Release workflow: `.github/workflows/release.yml`
- Publish workflow: `.github/workflows/publish.yml`
- semantic-release configuration: `.releaserc.yml`
- CI wrapper for semantic-release: `scripts/semantic-release.sh`
- Tag signing: `scripts/sign-release-tag.sh`
- Publish checks and artifacts: `scripts/publish.sh`

## Release Prerequisites

- Changes must be merged into `main`.
- Commit messages must follow Conventional Commits (semantic-release uses these to determine versions).
- GitHub secrets for release/publish must be configured (see `RELEASE.md`).

## Semantic-release Tags

### Git Release Tag Format

- Allowed pattern: `vMAJOR.MINOR.PATCH` (optionally with a suffix, e.g. `-rc.1`)
- `scripts/publish.sh` validates against this pattern:
  - `^v[0-9]+\.[0-9]+\.[0-9]+([.-].*)?$`

Examples:

- `v1.2.3`
- `v1.2.3-rc.1`
- `v1.2.3-beta.2`

### Commit Tags (Conventional Commits)

semantic-release (preset `conventionalcommits`) calculates the next release from commit types:

- `feat:` causes a **minor** bump
- `fix:` causes a **patch** bump
- `perf:` typically causes a **patch** bump
- `feat!:` or `fix!:` causes a **major** bump
- `BREAKING CHANGE:` in the footer causes a **major** bump
- `docs:`, `chore:`, `test:`, `refactor:` usually do not trigger a release unless a releasing type is also included

Examples:

- `feat(cli): add export command`
- `fix(core): handle empty values file`
- `feat(api)!: remove legacy endpoint`

```text
refactor(core): simplify resolver

BREAKING CHANGE: resolver output schema changed
```

## Release Flow (End-to-End)

1. Merge to `main` triggers `.github/workflows/release.yml`.
2. The job runs `./scripts/semantic-release.sh`.
3. semantic-release analyzes commit messages since the last tag.
4. In `prepare`, `CHANGELOG.md` and root `README.md` are updated and committed as the release commit.
5. semantic-release creates the release tag (initially lightweight).
6. `scripts/sign-release-tag.sh` rewrites the new tag as a **signed annotated tag** and pushes it.
7. The release workflow dispatches `hydra_publish` with tag and SHA.
8. `.github/workflows/publish.yml` checks out the tag and runs `./scripts/publish.sh`.
9. `publish.sh verify` validates signature, tagger identity, and that the tag points to `origin/main`.
10. Then artifact build and publish run for CLI and container.

## Why lightweight -> signed annotated?

The current flow is intentionally designed this way:

- semantic-release can reliably keep git-note metadata on the target commit.
- Directly creating signed tags has been more fragile in this setup.
- Therefore the newly created tag is signed and replaced immediately afterward.

## Local Validation (Without a Real Release)

- Check GoReleaser configuration:

  ```bash
  cd hydra-go
  goreleaser check --config .goreleaser.yml
  ```

- Optional semantic-release dry run:

  ```bash
  npx -y \
    -p semantic-release@25.0.3 \
    -p conventional-changelog-conventionalcommits \
    -p @semantic-release/changelog \
    -p @semantic-release/exec \
    -p @semantic-release/git \
    semantic-release --dry-run --debug
  ```

Note: A real release requires CI context, permissions, and secrets.

## Common Reasons for "No Release"

- There are no releasing commits since the last tag (`feat`, `fix`, `perf`, or breaking change).
- Commits do not follow Conventional Commit format.
- The workflow was not triggered from `main`.
- Missing permissions or secrets in the release/publish environment.