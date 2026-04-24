from __future__ import annotations

import logging
import signal
from datetime import datetime
from zoneinfo import ZoneInfo

import pika
from psycopg import Connection

from .builder import build_digest_data
from .db import (
    already_sent_today,
    fetch_pending_rows,
    insert_digest_articles,
    insert_pending_digest,
    mark_digest_failed,
    mark_digest_sent,
)
from .renderer import render_digest_html
from .sender import DigestSender

log = logging.getLogger(__name__)

TRIGGER_QUEUE = "digest.manual_trigger"
TRIGGER_QUEUE_TTL_MS = 60_000
TICK_SECONDS = 10


class DigestWorker:
    def __init__(
        self,
        *,
        rabbit_url: str,
        pg_conn: Connection,
        sender: DigestSender,
        tz: ZoneInfo,
        send_hour: int,
        recipient: str,
        from_addr: str,
        subject_prefix: str,
        record_in_db: bool,
    ) -> None:
        self.rabbit_url = rabbit_url
        self.pg_conn = pg_conn
        self.sender = sender
        self.tz = tz
        self.send_hour = send_hour
        self.recipient = recipient
        self.from_addr = from_addr
        self.subject_prefix = subject_prefix
        self.record_in_db = record_in_db
        self._shutdown = False

    def run(self) -> None:
        signal.signal(signal.SIGINT, self._on_signal)
        signal.signal(signal.SIGTERM, self._on_signal)

        params = pika.URLParameters(self.rabbit_url)
        params.heartbeat = 120
        connection = pika.BlockingConnection(params)
        channel = connection.channel()
        channel.queue_declare(
            queue=TRIGGER_QUEUE,
            durable=False,
            auto_delete=False,
            arguments={"x-message-ttl": TRIGGER_QUEUE_TTL_MS},
        )
        channel.basic_qos(prefetch_count=1)

        def on_trigger(ch, method, _props, _body):
            log.info("manual trigger received")
            try:
                self._run_digest(manual=True)
                ch.basic_ack(method.delivery_tag)
            except Exception:
                log.exception("manual digest failed")
                ch.basic_nack(method.delivery_tag, requeue=False)

        channel.basic_consume(queue=TRIGGER_QUEUE, on_message_callback=on_trigger)
        log.info(
            "digest worker started",
            extra={
                "send_hour": self.send_hour,
                "tz": str(self.tz),
                "recipient": self.recipient,
                "trigger_queue": TRIGGER_QUEUE,
                "record_in_db": self.record_in_db,
            },
        )

        try:
            while not self._shutdown:
                connection.process_data_events(time_limit=TICK_SECONDS)
                if self._shutdown:
                    break
                try:
                    self._check_scheduled()
                except Exception:
                    log.exception("scheduled check failed")
        finally:
            try:
                channel.close()
            except Exception:
                pass
            try:
                connection.close()
            except Exception:
                pass

    def _on_signal(self, sig, _frame) -> None:
        log.info("shutdown signal", extra={"signal": sig})
        self._shutdown = True

    # --- factored per-user guidance: scheduled needs both, manual needs only has_work ---

    def _is_scheduled_time(self, now_local: datetime) -> bool:
        return now_local.hour == self.send_hour

    def _has_work_to_do(self, local_date) -> bool:
        if self.record_in_db and already_sent_today(self.pg_conn, local_date):
            return False
        return len(fetch_pending_rows(self.pg_conn)) > 0

    def _check_scheduled(self) -> None:
        now_local = datetime.now(self.tz)
        if not self._is_scheduled_time(now_local):
            return
        if not self._has_work_to_do(now_local.date()):
            return
        self._run_digest(manual=False)

    # --- digest execution ---

    def _run_digest(self, manual: bool) -> None:
        now_local = datetime.now(self.tz)
        local_date = now_local.date()

        # Manual path bypasses the "scheduled hour" check but still respects
        # "already sent today" and "has pending articles" (per user spec).
        if not manual and already_sent_today(self.pg_conn, local_date) and self.record_in_db:
            log.info("digest already sent today, skipping", extra={"local_date": str(local_date)})
            return

        rows = fetch_pending_rows(self.pg_conn)
        if not rows:
            log.info("no pending articles, skipping", extra={"manual": manual})
            return

        data = build_digest_data(rows, local_date=local_date, sent_at=now_local)
        html = render_digest_html(data)
        subject = f"{self.subject_prefix} — {local_date.isoformat()}"

        if not self.record_in_db:
            # File mode: skip DB writes so the same articles stay pending for
            # continued template iteration.
            self.sender.send(
                subject=subject, html=html, recipient=self.recipient, from_addr=self.from_addr
            )
            log.info(
                "preview render complete",
                extra={"articles": data.total_articles, "queries": data.total_queries},
            )
            return

        digest_id = insert_pending_digest(self.pg_conn, manual=manual)
        try:
            self.sender.send(
                subject=subject, html=html, recipient=self.recipient, from_addr=self.from_addr
            )
        except Exception as e:
            mark_digest_failed(self.pg_conn, digest_id, str(e))
            log.exception("digest send failed", extra={"digest_id": digest_id, "manual": manual})
            raise

        mark_digest_sent(self.pg_conn, digest_id, local_date, data.total_articles)
        insert_digest_articles(self.pg_conn, digest_id, data.pmids)
        log.info(
            "digest sent",
            extra={
                "digest_id": digest_id,
                "articles": data.total_articles,
                "queries": data.total_queries,
                "manual": manual,
                "local_date": str(local_date),
            },
        )
