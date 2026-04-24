from __future__ import annotations

from typing import Protocol

from .db import QueryConfig


class Filterable(Protocol):
    abstract: str | None
    publication_types: list[str]


def passes_filters(article: Filterable, query: QueryConfig) -> tuple[bool, str | None]:
    """Returns (passes, reason_if_rejected).

    Rules applied in order:
    1. Abstract length >= query.min_abstract_length.
    2. If query.publication_type_allowlist is set (non-null), article must have
       at least one type in the allowlist.
    3. No publication type in query.publication_type_blocklist is allowed.
    """
    abstract = article.abstract or ""
    if len(abstract) < query.min_abstract_length:
        return False, f"abstract_too_short ({len(abstract)}<{query.min_abstract_length})"

    pub_types = set(article.publication_types or [])

    if query.publication_type_allowlist is not None:
        allowed = set(query.publication_type_allowlist)
        if not pub_types & allowed:
            return False, f"no_allowed_pub_type (have={sorted(pub_types)}, want_any_of={sorted(allowed)})"

    blocked = set(query.publication_type_blocklist or [])
    hit = pub_types & blocked
    if hit:
        return False, f"blocked_pub_type ({sorted(hit)})"

    return True, None
