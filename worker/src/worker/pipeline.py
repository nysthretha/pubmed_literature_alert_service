from __future__ import annotations

import logging

from psycopg import Connection

from .db import (
    article_exists,
    get_query_config,
    get_stored_article,
    insert_article,
    insert_query_match,
)
from .filters import passes_filters
from .parser import parse_pubmed_xml
from .pubmed_client import EFetchClient

log = logging.getLogger(__name__)


def process(conn: Connection, efetch: EFetchClient, pmid: str, query_id: int) -> None:
    """Per-message orchestration: parse/fetch → filter → insert.

    Rule: rejected articles are still stored in `articles` (so another query
    can dedup against them), but `query_matches` is only inserted when the
    article passes this query's filters.
    """
    query = get_query_config(conn, query_id)
    if query is None:
        log.error("unknown query_id, dropping", extra={"pmid": pmid, "query_id": query_id})
        return

    if article_exists(conn, pmid):
        stored = get_stored_article(conn, pmid)
        if stored is None:
            log.warning("article vanished between exists-check and read", extra={"pmid": pmid})
            return
        ok, reason = passes_filters(stored, query)
        if ok:
            with conn.transaction():
                insert_query_match(conn, query_id, pmid)
            log.info("dedup_match", extra={"pmid": pmid, "query_id": query_id, "query": query.name})
        else:
            log.info("dedup_reject", extra={"pmid": pmid, "query_id": query_id, "query": query.name, "reason": reason})
        return

    xml_bytes = efetch.fetch(pmid)
    article = parse_pubmed_xml(xml_bytes, pmid)
    raw_xml = xml_bytes.decode("utf-8", errors="replace")

    ok, reason = passes_filters(article, query)
    with conn.transaction():
        inserted = insert_article(conn, article, raw_xml=raw_xml)
        if ok:
            insert_query_match(conn, query_id, pmid)

    if ok:
        log.info(
            "stored",
            extra={
                "pmid": pmid,
                "query_id": query_id,
                "query": query.name,
                "new": inserted,
                "title": article.title[:80],
            },
        )
    else:
        log.info(
            "stored_rejected",
            extra={
                "pmid": pmid,
                "query_id": query_id,
                "query": query.name,
                "new": inserted,
                "reason": reason,
            },
        )
