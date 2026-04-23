from __future__ import annotations

import json
import os
import signal
import sys
import time

import pika
import psycopg
from psycopg import Connection

from .db import article_exists, insert_article, insert_query_match
from .logging_setup import get_logger, setup_logging
from .pubmed import EFetchClient, parse_pubmed_xml

FETCH_QUEUE = "pmid.fetch"

log = get_logger(__name__)


def connect_rabbit(url: str) -> pika.BlockingConnection:
    last_err: Exception | None = None
    for attempt in range(1, 6):
        try:
            return pika.BlockingConnection(pika.URLParameters(url))
        except Exception as e:
            last_err = e
            log.warning("rabbitmq connect attempt failed", extra={"attempt": attempt, "error": str(e)})
            time.sleep(2)
    raise RuntimeError(f"rabbitmq connect failed: {last_err}")


def wait_for_schema(conn: Connection, max_attempts: int = 20) -> None:
    """Poll until the scheduler has applied migrations and the articles table exists."""
    for attempt in range(1, max_attempts + 1):
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT 1 FROM articles LIMIT 1")
            conn.rollback()
            return
        except psycopg.errors.UndefinedTable:
            conn.rollback()
            log.info("waiting for scheduler migrations", extra={"attempt": attempt})
            time.sleep(3)
    raise RuntimeError("articles table never appeared — is the scheduler running?")


def process(conn: Connection, efetch: EFetchClient, pmid: str, query_id: int) -> None:
    if article_exists(conn, pmid):
        with conn.transaction():
            insert_query_match(conn, query_id, pmid)
        log.info("dedup", extra={"pmid": pmid, "query_id": query_id})
        return

    xml_bytes = efetch.fetch(pmid)
    article = parse_pubmed_xml(xml_bytes, pmid)

    with conn.transaction():
        inserted = insert_article(conn, article, raw_xml=xml_bytes.decode("utf-8", errors="replace"))
        insert_query_match(conn, query_id, pmid)

    log.info("stored", extra={"pmid": pmid, "query_id": query_id, "new": inserted, "title": article.title[:80]})


def main() -> int:
    setup_logging()

    rabbit_url = os.environ["RABBITMQ_URL"]
    pg_url = os.environ["POSTGRES_URL"]
    email = os.environ["PUBMED_EMAIL"]
    tool = os.environ.get("PUBMED_TOOL_NAME", "pubmed-alerts")
    api_key = os.environ.get("PUBMED_API_KEY") or None

    pg_conn = psycopg.connect(pg_url, autocommit=True)
    wait_for_schema(pg_conn)

    efetch = EFetchClient(tool=tool, email=email, api_key=api_key)

    connection = connect_rabbit(rabbit_url)
    channel = connection.channel()
    channel.queue_declare(queue=FETCH_QUEUE, durable=True)
    channel.basic_qos(prefetch_count=1)

    def on_message(ch, method, _props, body: bytes) -> None:
        try:
            msg = json.loads(body)
            pmid = str(msg["pmid"])
            query_id = int(msg["query_id"])
        except (json.JSONDecodeError, KeyError, ValueError) as e:
            log.error("bad message, dropping", extra={"error": str(e)})
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
            return

        try:
            process(pg_conn, efetch, pmid, query_id)
            ch.basic_ack(delivery_tag=method.delivery_tag)
        except Exception as e:
            try:
                pg_conn.rollback()
            except Exception:
                pass
            log.exception("process failed, dropping message", extra={"pmid": pmid, "query_id": query_id, "error": str(e)})
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

    channel.basic_consume(queue=FETCH_QUEUE, on_message_callback=on_message)
    log.info("consuming", extra={"queue": FETCH_QUEUE, "api_key_present": bool(api_key)})

    def _shutdown(signum, _frame):
        log.info("shutdown signal", extra={"signal": signum})
        try:
            channel.stop_consuming()
        except Exception:
            pass

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    try:
        channel.start_consuming()
    finally:
        try:
            connection.close()
        except Exception:
            pass
        try:
            pg_conn.close()
        except Exception:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main())
