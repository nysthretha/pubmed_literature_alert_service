from __future__ import annotations

from importlib.resources import files

import jinja2
from markupsafe import Markup, escape

from .builder import DigestData


def _nl2br(s: str | None) -> Markup:
    return Markup(str(escape(s or "")).replace("\n", "<br>"))


def _load_env() -> jinja2.Environment:
    loader = jinja2.FunctionLoader(
        lambda name: files("worker.digest.templates").joinpath(name).read_text(encoding="utf-8")
    )
    env = jinja2.Environment(
        loader=loader,
        autoescape=jinja2.select_autoescape(["html", "j2"]),
        trim_blocks=True,
        lstrip_blocks=True,
    )
    env.filters["nl2br"] = _nl2br
    return env


_env = _load_env()


def render_digest_html(data: DigestData) -> str:
    tmpl = _env.get_template("digest.html.j2")
    return tmpl.render(data=data)
