# Tutorial 6: CI Pipeline

Learn to set up a Hydra CI pipeline for automated testing, releasing, and promoting.

## Overview

Hydra CI automates the chart lifecycle:

1. **Test** — Lint and validate charts
2. **Release** — Detect changes, bump versions, create tags
3. **Promote** — Create merge requests for environment promotion (dev → stage → prod)
4. **Package** — Build and push to OCI registry

## Step 1: Create Configuration

Use the interactive config generator:

```bash
hydra ci config .hydra-ci.yaml
```

This creates a `.hydra-ci.yaml` file with your repository's pipeline configuration.

## Step 2: Run Tests Locally

```bash
hydra ci run test .hydra-ci.yaml --local --dry-run
```

The `--local` flag commits but does not push or create merge requests. The `--dry-run` flag simulates without any changes.

## Step 3: Test Without Dry-Run

```bash
hydra ci run test .hydra-ci.yaml --local
```

Validates all charts pass linting and template rendering.

## Step 4: Release

```bash
hydra ci run release .hydra-ci.yaml --local
```

Detects changed charts, bumps versions, creates Git tags.

## Step 5: Promote

```bash
hydra ci run promote .hydra-ci.yaml --local --promote-to stage
```

Creates a promote merge request targeting the stage environment.

## Step 6: Run All Stages

```bash
hydra ci run auto .hydra-ci.yaml --local
```

Runs all stages in dependency order, stopping at the first failure.

## Integrating with GitLab/GitHub

In your CI configuration, call Hydra without `--local`:

```yaml
# GitLab CI example
hydra-release:
  script:
    - hydra ci run release .hydra-ci.yaml
```

Without `--local`, Hydra pushes changes, creates tags, and opens merge requests.

## Additional CI Commands

| Command | Purpose |
|---------|---------|
| `hydra ci run sprint` | Bump major version at sprint boundary |
| `hydra ci run upgrade` | Update service version from service deployment pipeline |
| `hydra ci run sync` | Copy cluster configs into repository |
| `hydra ci run update` | Refresh unit test data |

## Next Steps

- [Workflow: CI/CD Integration](../workflows/ci-cd-integration.md)
- [Configuration: .hydra-ci.yaml](../configuration/hydra-ci-yaml.md)
- [Commands: CI](../commands/ci/)
