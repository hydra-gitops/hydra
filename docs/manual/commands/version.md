# hydra version

Print the Hydra CLI version.

## CLI help recording

<!-- markdownlint-disable MD033 -->
<div class="hydra-asciinema" data-cast-slug="version"></div>
<!-- markdownlint-enable MD033 -->

## Synopsis

```bash
hydra version
```

## Description

Prints a single line with the build version, for example `hydra v1.2.3`. Local development builds without release metadata report `hydra dev`.

Hydra skips the usual stderr welcome line for this command so stdout stays a single line suitable for scripts.

## Example

```bash
$ hydra version
hydra dev
```
