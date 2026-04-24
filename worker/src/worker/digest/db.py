from __future__ import annotations

from dataclasses import dataclass
from datetime import date

from psycopg import Connection


@dataclass
class PendingRow:
    """One row per (article, matched query) pair for articles not yet in any digest."""
    pmid: str
    title: str
    abstract: str | None
    journal: str | None
    publication_date: date | None
    authors: list[dict]
    query_id: int
    query_name: str
    query_notes: str | None


def already_sent_today(conn: Connection, local_date: date) -> bool:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT 1 FROM digests WHERE sent_local_date = %s AND status = 'sent'",
            (local_date,),
        )
        return cur.fetchone() is not None


def fetch_pending_rows(conn: Connection) -> list[PendingRow]:
    """Articles not yet included in any digest, joined with their query matches.
    Rejected articles (no query_matches row) are naturally excluded by the INNER JOIN.
    """
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT a.pmid, a.title, a.abstract, a.journal, a.publication_date, a.authors,
                   q.id, q.name, q.notes
            FROM articles a
            JOIN query_matches qm ON qm.pmid = a.pmid
            JOIN queries q ON q.id = qm.query_id
            WHERE NOT EXISTS (SELECT 1 FROM digest_articles da WHERE da.pmid = a.pmid)
            ORDER BY q.name, a.publication_date DESC NULLS LAST, a.pmid
            """
        )
        rows = cur.fetchall()
    return [
        PendingRow(
            pmid=r[0], title=r[1], abstract=r[2], journal=r[3],
            publication_date=r[4], authors=list(r[5] or []),
            query_id=r[6], query_name=r[7], query_notes=r[8],
        )
        for r in rows
    ]


def insert_pending_digest(conn: Connection, manual: bool) -> int:
    with conn.cursor() as cur:
        cur.execute(
            "INSERT INTO digests (status, manual) VALUES ('pending', %s) RETURNING id",
            (manual,),
        )
        return cur.fetchone()[0]


def mark_digest_sent(
    conn: Connection, digest_id: int, local_date: date, articles_included: int
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE digests
            SET status = 'sent', sent_local_date = %s, articles_included = %s
            WHERE id = %s
            """,
            (local_date, articles_included, digest_id),
        )


def mark_digest_failed(conn: Connection, digest_id: int, error: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            "UPDATE digests SET status = 'failed', error_message = %s WHERE id = %s",
            (error[:2000], digest_id),
        )


def insert_digest_articles(conn: Connection, digest_id: int, pmids: set[str]) -> None:
    if not pmids:
        return
    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO digest_articles (digest_id, pmid) VALUES (%s, %s) "
            "ON CONFLICT DO NOTHING",
            [(digest_id, p) for p in pmids],
        )
