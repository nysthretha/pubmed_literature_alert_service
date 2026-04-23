from __future__ import annotations

import json

from psycopg import Connection

from .pubmed import Article


def article_exists(conn: Connection, pmid: str) -> bool:
    with conn.cursor() as cur:
        cur.execute("SELECT 1 FROM articles WHERE pmid = %s", (pmid,))
        return cur.fetchone() is not None


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
