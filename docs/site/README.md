# Hydra user manual — static site

MkDocs Material site for [https://hydra-gitops.org/](https://hydra-gitops.org/).

| Path | Role |
| ---- | ---- |
| `../manual/` | Markdown source (not copied; read at build time) |
| `../asciinema/help/` | Terminal recordings (`.cast`) for command help |
| `hydra_mkdocs/` | MkDocs plugin (navigation, asciinema embeds) |
| `site/` | Build output (generated; do not edit) |

## Prerequisites

- **Python 3.10+** (`python3 --version`)
- Network access on first run (pip downloads MkDocs and dependencies)

## Quick test (build + static preview)

From the repository root:

```bash
cd hydra/docs/site
./build.sh
python3 -m http.server --directory site 8080
```

Open [http://127.0.0.1:8080/](http://127.0.0.1:8080/) in a browser.

**What to check**

- Home page loads and the left navigation lists all manual chapters.
- Open a command page, e.g. **Commands → Local → hydra local inspect**.
- Section **CLI help recording** shows an asciinema player; press play to run the cast.
- Workflow page **Workflows → Workflow: CI Pipeline** renders the Mermaid diagram.

Stop the preview server with `Ctrl+C`.

`./build.sh` creates a virtualenv in `.venv/`, installs dependencies, and writes HTML to `site/`.

## Live reload while editing (recommended)

```bash
cd hydra/docs/site
./test.sh
```

Opens [http://127.0.0.1:8000/](http://127.0.0.1:8000/) with live reload when you edit files under `../manual/`. Stop with `Ctrl+C`.

Changes to **asciinema casts** (`../asciinema/help/*.cast`) and **site assets** (player, `asciinema-embed.js`) trigger a rebuild automatically — you do not need to restart `./test.sh` after `hydra record help`. Use a hard refresh in the browser if a script change does not appear.

`./test.sh` creates `.venv/` on first run, installs dependencies, then runs `mkdocs serve`. Extra arguments are passed through, for example:

```bash
./test.sh -a 127.0.0.1:9000
```

## Rebuild without recreating the venv

```bash
cd hydra/docs/site
source .venv/bin/activate
mkdocs build
python3 -m http.server --directory site 8080
```

## Clean rebuild

```bash
cd hydra/docs/site
rm -rf site/ .venv/
./build.sh
```

## Asciinema player

Bundled **asciinema-player 3.15.1** (`assets/asciinema-player/`). Recordings use **asciicast v3** (`"version": 3` in `.cast` headers); the player must be **≥ 3.10.0** to play them.

Help recordings **autoplay** on load (`autoPlay: true`). `controls: true` keeps the **timeline scrubber** visible. While paused: `,` / `.` step frame-by-frame (`minFrameTime: 0`), arrow keys seek ±5s.

## Asciinema recordings

Command pages under `../manual/commands/` get a player when:

1. The page H1 is `# hydra <command path>` (e.g. `# hydra local inspect`).
2. A file `../asciinema/help/<slug>.cast` exists (`local-inspect.cast` for that example).

Record or refresh casts from the repo root with a built `hydra` binary:

```bash
hydra record help
# or: hydra record help --output-dir hydra/docs/asciinema/help
```

Then rebuild or restart `mkdocs serve`.

## Production deploy

GitHub Actions workflow: `.github/workflows/docs-manual.yml`  
Publishes `hydra/docs/site/site/` to GitHub Pages with custom domain `hydra-gitops.org` (`extra/CNAME`).

## Troubleshooting

| Problem | What to try |
| ------- | ----------- |
| `python3: command not found` | Install Python 3.10+ or use `python` if it points to 3.10+ |
| `pip install` fails | Upgrade pip: `pip install --upgrade pip` |
| Player empty / 404 on `.cast` | Run `mkdocs build` again; check `site/asciinema/help/<slug>.cast` exists |
| Old content in browser | Hard refresh (`Ctrl+Shift+R`) or use `mkdocs serve` instead of a stale `site/` |
| MkDocs warning about links | Many manual links target develop docs not included in this site yet; build still succeeds |

## Layout of this directory

```text
hydra/docs/site/
├── README.md           ← this file
├── mkdocs.yml          ← site configuration
├── build.sh            ← one-shot local build script
├── test.sh             ← mkdocs serve (live reload)
├── requirements.txt
├── pyproject.toml
├── hydra_mkdocs/       ← plugin package
├── assets/             ← asciinema player (bundled)
├── javascripts/
├── stylesheets/
├── extra/CNAME         → copied to site root on build
└── site/               ← generated HTML (gitignored)
```
