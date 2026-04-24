from __future__ import annotations

import json
import logging
import time
from dataclasses import dataclass

import psycopg
from psycopg import Connection

from .parser import Article

log = logging.getLogger(__name__)


@dataclass
class QueryConfig:
    id: int
    name: str
    min_abstract_length: int
    publication_type_allowlist: list[str] | None
    publication_type_blocklist: list[str]


@dataclass
class StoredArticle:
    """Minimal view of a stored article for running filter checks on the dedup path."""
    abstract: str | None
    publication_types: list[str]


def connect_db(url: str) -> Connection:
    """autocommit=True is required — conn.transaction() blocks are our BEGIN/COMMIT boundaries."""
    return psycopg.connect(url, autocommit=True)


def wait_for_schema(conn: Connection, max_attempts: int = 20) -> None:
    """Poll information_schema until migration 00003 has landed (min_abstract_length column)."""
    for attempt in range(1, max_attempts + 1):
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT 1 FROM information_schema.columns
                    WHERE table_name = 'queries' AND column_name = 'min_abstract_length'
                    """
                )
                if cur.fetchone() is not None:
                    return
        except Exception as e:
            log.warning("schema check errored, retrying", extra={"attempt": attempt, "error": str(e)})
        log.info("waiting for scheduler migrations", extra={"attempt": attempt})
        time.sleep(3)
    raise RuntimeError("schema never reached expected state — is the scheduler running?")


def get_query_config(conn: Connection, query_id: int) -> QueryConfig | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id, name, min_abstract_length,
                   publication_type_allowlist, publication_type_blocklist
            FROM queries
            WHERE id = %s
            """,
            (query_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return QueryConfig(
        id=row[0],
        name=row[1],
        min_abstract_length=row[2],
        publication_type_allowlist=row[3],
        publication_type_blocklist=row[4] or [],
    )


def article_exists(conn: Connection, pmid: str) -> bool:
    with conn.cursor() as cur:
        cur.execute("SELECT 1 FROM articles WHERE pmid = %s", (pmid,))
        return cur.fetchone() is not None


def get_stored_article(conn: Connection, pmid: str) -> StoredArticle | None:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT abstract, publication_types FROM articles WHERE pmid = %s",
            (pmid,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return StoredArticle(abstract=row[0], publication_types=list(row[1] or []))


def insert_article(conn: Connection, article: Article, raw_xml: str) -> bool:
    """Returns True if a new row was inserted, False on conflict."""
    authors_json = json.dumps([a.__dict__ for a in article.authors])
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO articles
                (pmid, title, abstract, journal, publication_date,
                 authors, publication_types, raw_xml)
            VALUES (%s, %s, %s, %s, %s, %s::jsonb, %s, %s)
            ON CONFLICT (pmid) DO NOTHING
            """,
            (
                article.pmid,
                article.title,
                article.abstract,
                article.journal,
                article.publication_date,
                authors_json,
                article.publication_types,
                raw_xml,
            ),
        )
        return cur.rowcount == 1


def insert_query_match(conn: Connection, query_id: int, pmid: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO query_matches (query_id, pmid)
            VALUES (%s, %s)
            ON CONFLICT DO NOTHING
            """,
            (query_id, pmid),
        )
