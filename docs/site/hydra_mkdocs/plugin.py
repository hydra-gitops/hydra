"""MkDocs plugin: navigation generation and asciinema embeds on command pages."""

from __future__ import annotations

import logging
import re
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from mkdocs.config.defaults import MkDocsConfig
    from mkdocs.livereload import LiveReloadServer

log = logging.getLogger("mkdocs.plugins.hydra_manual")

from mkdocs.config import config_options
from mkdocs.plugins import BasePlugin
from mkdocs.structure.files import File, InclusionLevel
from mkdocs.structure.pages import Page

# Paths in mkdocs.yml extra_css / extra_javascript (relative to docs_dir URLs).
# Physical files live next to mkdocs.yml; registered via on_files.
_SITE_ASSETS = (
    "assets/asciinema-player/asciinema-player.css",
    "assets/asciinema-player/asciinema-player.min.js",
    "stylesheets/extra.css",
    "javascripts/asciinema-embed.js",
)

_COMMAND_H1 = re.compile(r"^#\s+hydra\s+(.+)$", re.MULTILINE)
_SKIP_COMMAND_PAGES = frozenset(
    {
        "commands/README.md",
        "commands/inspect-shared.md",
        "commands/app-id-patterns.md",
    }
)

_SECTION_TITLES = {
    "cel": "CEL",
    "ci": "CI",
    "configuration": "Configuration",
    "introduction": "Introduction",
    "migration": "Migration",
    "presets": "Presets",
    "refs": "Refs",
    "tutorials": "Tutorials",
    "values": "Values",
    "workflows": "Workflows",
    "concepts": "Concepts",
    "commands": "Commands",
    "argocd": "ArgoCD",
    "cluster": "GitOps (cluster)",
    "local": "Local",
}

# Top-level manual chapters after Home (CEL last).
_TOP_LEVEL_NAV_ORDER = (
    "introduction",
    "configuration",
    "concepts",
    "tutorials",
    "values",
    "refs",
    "presets",
    "commands",
    "workflows",
    "migration",
    "cel",
)


class HydraManualPlugin(BasePlugin):
    """Build nav from the manual tree and embed help recordings on command pages."""

    config_scheme = (
        ("asciinema_source", config_options.Type(str, default="../asciinema/help")),
        ("asciinema_site_path", config_options.Type(str, default="asciinema/help")),
    )

    def on_config(self, config) -> None:
        docs_dir = Path(config.docs_dir)
        if not config.nav:
            config.nav = _build_nav(docs_dir)

    def on_files(self, files, *, config):
        site_root = Path(config.config_file_path).parent.resolve()
        _register_static_files(files, config, site_root, _SITE_ASSETS)
        _register_asciinema_casts(files, config, site_root, self.config["asciinema_source"], self.config["asciinema_site_path"])
        return files

    def on_serve(
        self,
        server: "LiveReloadServer",
        /,
        *,
        config: "MkDocsConfig",
        builder,
    ) -> "LiveReloadServer":
        site_root = Path(config.config_file_path).parent.resolve()
        casts_dir = (site_root / self.config["asciinema_source"]).resolve()
        if casts_dir.is_dir():
            server.watch(str(casts_dir))
        for dirname in ("assets", "stylesheets", "javascripts", "hydra_mkdocs"):
            path = site_root / dirname
            if path.is_dir():
                server.watch(str(path))
        return server

    def on_page_markdown(self, markdown: str, *, page: Page, config, files) -> str:
        if _has_asciinema_embed(markdown):
            return markdown
        rel = _page_relpath(page, config)
        slug = _command_cast_slug(rel, markdown, self._casts_available(config))
        if slug is None:
            return markdown
        return _inject_asciinema_block(markdown, slug)

    def on_post_build(self, *, config, **kwargs) -> None:
        site_root = Path(config.config_file_path).parent
        site_dir = Path(config.site_dir)

        cname = site_root / "extra" / "CNAME"
        if cname.is_file():
            (site_dir / "CNAME").write_text(
                cname.read_text(encoding="utf-8").strip() + "\n",
                encoding="utf-8",
            )

    def _casts_available(self, config) -> set[str]:
        source = Path(config.config_file_path).parent / self.config["asciinema_source"]
        if not source.is_dir():
            return set()
        return {p.stem for p in source.glob("*.cast")}


def _page_relpath(page: Page, config) -> str:
    docs_dir = Path(config.docs_dir).resolve()
    src = Path(page.file.abs_src_path).resolve()
    try:
        return src.relative_to(docs_dir).as_posix()
    except ValueError:
        return Path(page.file.src_path).as_posix()


def _register_static_files(files, config, site_root: Path, rel_paths: tuple[str, ...]) -> None:
    for src_uri in rel_paths:
        abs_path = site_root / src_uri
        if not abs_path.is_file():
            log.warning("site asset missing: %s", abs_path)
            continue
        if files.get_file_from_path(src_uri) is not None:
            continue
        files.append(
            File.generated(
                config,
                src_uri,
                abs_src_path=str(abs_path),
                inclusion=InclusionLevel.NOT_IN_NAV,
            )
        )


def _register_asciinema_casts(
    files,
    config,
    site_root: Path,
    asciinema_source: str,
    asciinema_site_path: str,
) -> None:
    casts_dir = site_root / asciinema_source
    if not casts_dir.is_dir():
        return
    prefix = asciinema_site_path.rstrip("/")
    for cast in sorted(casts_dir.glob("*.cast")):
        src_uri = f"{prefix}/{cast.name}"
        if files.get_file_from_path(src_uri) is not None:
            continue
        files.append(
            File.generated(
                config,
                src_uri,
                abs_src_path=str(cast.resolve()),
                inclusion=InclusionLevel.NOT_IN_NAV,
            )
        )


def _has_asciinema_embed(markdown: str) -> bool:
    return "hydra-asciinema" in markdown or "## CLI help recording" in markdown


def _command_cast_slug(rel_path: str, markdown: str, casts: set[str]) -> str | None:
    if not rel_path.startswith("commands/") or not rel_path.endswith(".md"):
        return None
    if rel_path in _SKIP_COMMAND_PAGES:
        return None
    match = _COMMAND_H1.search(markdown)
    if not match:
        return None
    slug = match.group(1).strip().replace(" ", "-")
    if slug not in casts:
        return None
    return slug


def _inject_asciinema_block(markdown: str, slug: str) -> str:
    block = (
        "\n\n## CLI help recording\n\n"
        f'<div class="hydra-asciinema" data-cast-slug="{slug}"></div>\n\n'
    )
    synopsis = "\n## Synopsis\n"
    if synopsis in markdown:
        return markdown.replace(synopsis, block + synopsis, 1)
    lines = markdown.splitlines(keepends=True)
    for idx, line in enumerate(lines):
        if line.startswith("# "):
            return "".join(lines[: idx + 1]) + block + "".join(lines[idx + 1 :])
    return markdown + block


def _build_nav(docs_dir: Path) -> list:
    nav: list = [{"Home": "README.md"}]
    dirs_by_name = {
        child.name: child
        for child in docs_dir.iterdir()
        if child.is_dir() and not child.name.startswith(".")
    }
    ordered_names = list(_TOP_LEVEL_NAV_ORDER) + sorted(
        dirs_by_name.keys() - set(_TOP_LEVEL_NAV_ORDER)
    )
    for name in ordered_names:
        child = dirs_by_name.get(name)
        if child is None:
            continue
        section = _build_nav_dir(child, docs_dir)
        if section:
            title = _SECTION_TITLES.get(name, _title_case(name))
            nav.append({title: section})
    return nav


def _build_nav_dir(directory: Path, docs_root: Path) -> list:
    entries: list = []
    readme = directory / "README.md"
    if readme.is_file():
        entries.append({_page_title(readme): _rel_path(readme, docs_root)})

    for md in sorted(directory.glob("*.md")):
        if md.name == "README.md":
            continue
        entries.append({_page_title(md): _rel_path(md, docs_root)})

    for child in sorted(directory.iterdir(), key=lambda p: p.name):
        if not child.is_dir() or child.name.startswith("."):
            continue
        subsection = _build_nav_dir(child, docs_root)
        if subsection:
            title = _SECTION_TITLES.get(child.name, _title_case(child.name))
            entries.append({title: subsection})
    return entries


def _rel_path(path: Path, docs_root: Path) -> str:
    return path.resolve().relative_to(docs_root.resolve()).as_posix()


def _page_title(path: Path) -> str:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return _title_case(path.stem)
    match = re.search(r"^#\s+(.+)$", text, re.MULTILINE)
    if match:
        title = match.group(1).strip()
        if title.lower().startswith("hydra "):
            return title
        return title
    return _title_case(path.stem)


def _title_case(name: str) -> str:
    return name.replace("-", " ").replace("_", " ").title()
