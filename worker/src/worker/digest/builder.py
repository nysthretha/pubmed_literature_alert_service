from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime
from typing import Iterable

from .db import PendingRow


@dataclass
class DigestArticle:
    pmid: str
    title: str
    abstract: str | None
    journal: str | None
    publication_date: date | None
    authors_summary: str


@dataclass
class DigestGroup:
    query_id: int
    query_name: str
    query_notes: str | None
    articles: list[DigestArticle]


@dataclass
class DigestData:
    date: date
    sent_at: datetime
    total_articles: int
    total_queries: int
    groups: list[DigestGroup]
    pmids: set[str]


def format_authors(authors: list[dict]) -> str:
    """From stored JSONB authors, render 'Smith J, Jones A, Brown K et al.'"""
    if not authors:
        return "(no authors listed)"
    parts: list[str] = []
    for a in authors[:3]:
        if a.get("collective_name"):
            parts.append(a["collective_name"])
            continue
        last = a.get("last_name") or ""
        ini = a.get("initials") or ""
        if last and ini:
            parts.append(f"{last} {ini}")
        elif last:
            parts.append(last)
    out = ", ".join(parts) if parts else "(unnamed authors)"
    if len(authors) > 3:
        out += " et al."
    return out


def build_digest_data(
    rows: Iterable[PendingRow], local_date: date, sent_at: datetime
) -> DigestData:
    """Group (article, query) rows into sections by query.

    One article can appear under multiple sections if it matched multiple queries —
    readers want to see it in every relevant clinical context.
    """
    by_query: dict[int, DigestGroup] = {}
    all_pmids: set[str] = set()

    for row in rows:
        all_pmids.add(row.pmid)
        group = by_query.get(row.query_id)
        if group is None:
            group = DigestGroup(
                query_id=row.query_id,
                query_name=row.query_name,
                query_notes=row.query_notes,
                articles=[],
            )
            by_query[row.query_id] = group
        if any(a.pmid == row.pmid for a in group.articles):
            continue  # belt-and-braces: query_matches PK already prevents dup
        group.articles.append(
            DigestArticle(
                pmid=row.pmid,
                title=row.title,
                abstract=row.abstract,
                journal=row.journal,
                publication_date=row.publication_date,
                authors_summary=format_authors(row.authors),
            )
        )

    groups = sorted(by_query.values(), key=lambda g: g.query_name.lower())
    for g in groups:
        g.articles.sort(
            key=lambda a: (a.publication_date or date.min, a.pmid),
            reverse=True,
        )

    return DigestData(
        date=local_date,
        sent_at=sent_at,
        total_articles=len(all_pmids),
        total_queries=len(groups),
        groups=groups,
        pmids=all_pmids,
    )
