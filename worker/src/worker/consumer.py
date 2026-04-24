from __future__ import annotations

import json
import logging
import time

import pika
from psycopg import Connection

from .pipeline import process
from .pubmed_client import EFetchClient

FETCH_QUEUE = "pmid.fetch"

log = logging.getLogger(__name__)


class Consumer:
    def __init__(self, rabbit_url: str, pg_conn: Connection, efetch: EFetchClient):
        self.rabbit_url = rabbit_url
        self.pg_conn = pg_conn
        self.efetch = efetch
        self.connection: pika.BlockingConnection | None = None
        self.channel = None

    def run(self) -> None:
        self.connection = self._connect_rabbit()
        self.channel = self.connection.channel()
        self.channel.queue_declare(queue=FETCH_QUEUE, durable=True)
        self.channel.basic_qos(prefetch_count=1)
        self.channel.basic_consume(queue=FETCH_QUEUE, on_message_callback=self._on_message)
        log.info("consuming", extra={"queue": FETCH_QUEUE, "api_key_present": bool(self.efetch.api_key)})
        try:
            self.channel.start_consuming()
        finally:
            try:
                if self.connection is not None:
                    self.connection.close()
            except Exception:
                pass

    def stop(self) -> None:
        try:
            if self.channel is not None:
                self.channel.stop_consuming()
        except Exception:
            pass

    def _connect_rabbit(self) -> pika.BlockingConnection:
        last_err: Exception | None = None
        for attempt in range(1, 6):
            try:
                return pika.BlockingConnection(pika.URLParameters(self.rabbit_url))
            except Exception as e:
                last_err = e
                log.warning("rabbitmq connect attempt failed", extra={"attempt": attempt, "error": str(e)})
                time.sleep(2)
        raise RuntimeError(f"rabbitmq connect failed: {last_err}")

    def _on_message(self, ch, method, _props, body: bytes) -> None:
        try:
            msg = json.loads(body)
            pmid = str(msg["pmid"])
            query_id = int(msg["query_id"])
        except (json.JSONDecodeError, KeyError, ValueError) as e:
            log.error("bad message, dropping", extra={"error": str(e)})
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
            return

        try:
            process(self.pg_conn, self.efetch, pmid, query_id)
            ch.basic_ack(delivery_tag=method.delivery_tag)
        except Exception as e:
            log.exception("process failed, dropping", extra={"pmid": pmid, "query_id": query_id, "error": str(e)})
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
