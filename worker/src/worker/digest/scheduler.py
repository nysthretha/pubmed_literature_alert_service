from __future__ import annotations

import logging
import signal
import time
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

import pika
import pika.exceptions
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

# Pika tunings — overridden from server defaults so we fail fast and visibly
# when the connection rots, instead of wedging silently for 30+ hours (which
# happened in production once and motivated this whole refactor).
PIKA_HEARTBEAT_SECONDS = 30
PIKA_BLOCKED_CONNECTION_TIMEOUT = 300

# How long to wait before retrying after an AMQP connection error. Prevents a
# hot reconnect spin if RabbitMQ is genuinely down.
RECONNECT_BACKOFF_SECONDS = 5

# Hourly progress signal so silent death turns into a visible signal — if no
# tick log appears for >hour, the worker is wedged and operator can act.
HEARTBEAT_LOG_INTERVAL_SECONDS = 3600


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
        self._connection: pika.BlockingConnection | None = None
        self._channel = None  # pika.adapters.blocking_connection.BlockingChannel
        self._last_heartbeat_at: datetime | None = None

    # --- connection lifecycle ----------------------------------------------

    def _connect(self) -> None:
        """Open a new AMQP connection + channel, declare the trigger queue,
        register the consumer. Idempotent — closes any prior connection
        first, so this doubles as the reconnect path."""
        # Best-effort cleanup of any prior state. We don't trust the old
        # connection's close() to succeed (we're here because it might be
        # broken), but try anyway so RabbitMQ sees a clean disconnect when
        # possible.
        if self._channel is not None:
            try:
                self._channel.close()
            except Exception:
                pass
        if self._connection is not None:
            try:
                self._connection.close()
            except Exception:
                pass

        params = pika.URLParameters(self.rabbit_url)
        params.heartbeat = PIKA_HEARTBEAT_SECONDS
        params.blocked_connection_timeout = PIKA_BLOCKED_CONNECTION_TIMEOUT

        self._connection = pika.BlockingConnection(params)
        self._channel = self._connection.channel()
        self._channel.queue_declare(
            queue=TRIGGER_QUEUE,
            durable=False,
            auto_delete=False,
            arguments={"x-message-ttl": TRIGGER_QUEUE_TTL_MS},
        )
        self._channel.basic_qos(prefetch_count=1)
        self._channel.basic_consume(
            queue=TRIGGER_QUEUE, on_message_callback=self._on_trigger
        )

    def _ensure_connected(self) -> bool:
        """If the AMQP connection or channel is not open, attempt to reconnect.
        Returns True if connected (or successfully reconnected), False if the
        attempt failed and the caller should back off. Never raises."""
        if (
            self._connection is not None
            and self._connection.is_open
            and self._channel is not None
            and self._channel.is_open
        ):
            return True

        log.warning("rabbitmq connection lost, reconnecting")
        try:
            self._connect()
        except pika.exceptions.AMQPConnectionError as e:
            log.error("rabbitmq reconnect failed", extra={"error": str(e)})
            return False
        except Exception as e:
            # AMQPChannelError, network errors not wrapped as AMQPConnectionError
            log.error("rabbitmq reconnect failed (unexpected)", extra={"error": str(e)})
            return False

        log.info("rabbitmq reconnected")
        return True

    def _on_trigger(self, ch, method, _props, _body) -> None:
        log.info("manual trigger received")
        try:
            self._run_digest(manual=True)
            ch.basic_ack(method.delivery_tag)
        except Exception:
            log.exception("manual digest failed")
            try:
                ch.basic_nack(method.delivery_tag, requeue=False)
            except Exception:
                # Channel may have died between recv and nack — let the
                # main loop's health check pick it up next iteration.
                pass

    # --- main loop ---------------------------------------------------------

    def run(self) -> None:
        signal.signal(signal.SIGINT, self._on_signal)
        signal.signal(signal.SIGTERM, self._on_signal)

        try:
            self._connect()
        except pika.exceptions.AMQPConnectionError as e:
            log.error("rabbitmq initial connect failed", extra={"error": str(e)})
            # Don't exit — main loop will retry via _ensure_connected.

        log.info(
            "digest worker started",
            extra={
                "send_hour": self.send_hour,
                "tz": str(self.tz),
                "recipient": self.recipient,
                "trigger_queue": TRIGGER_QUEUE,
                "record_in_db": self.record_in_db,
                "heartbeat_seconds": PIKA_HEARTBEAT_SECONDS,
            },
        )

        try:
            while not self._shutdown:
                # Health check before pumping events — a closed connection
                # would otherwise leave us blocked in process_data_events
                # waiting for I/O that never arrives.
                if not self._ensure_connected():
                    if self._shutdown:
                        break
                    time.sleep(RECONNECT_BACKOFF_SECONDS)
                    continue

                try:
                    self._connection.process_data_events(time_limit=TICK_SECONDS)
                except pika.exceptions.AMQPConnectionError as e:
                    log.error("rabbitmq drop during process_data_events", extra={"error": str(e)})
                    # Loop iteration will reconnect on the next pass.
                    continue
                except Exception:
                    log.exception("unexpected error in process_data_events")
                    continue

                if self._shutdown:
                    break

                try:
                    self._check_scheduled()
                except Exception:
                    log.exception("scheduled check failed")

                self._maybe_log_heartbeat()
        finally:
            try:
                if self._channel is not None:
                    self._channel.close()
            except Exception:
                pass
            try:
                if self._connection is not None:
                    self._connection.close()
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

    def _maybe_log_heartbeat(self) -> None:
        """Emit a tick log once per HEARTBEAT_LOG_INTERVAL_SECONDS so silent
        wedges become visible — operators can grep for `digest worker tick`
        and see continuity."""
        now_local = datetime.now(self.tz)
        interval = timedelta(seconds=HEARTBEAT_LOG_INTERVAL_SECONDS)
        if (
            self._last_heartbeat_at is None
            or now_local - self._last_heartbeat_at >= interval
        ):
            local_date = now_local.date()
            scheduled = self._is_scheduled_time(now_local)
            # has_work_to_do hits the DB; cheap but not free, and only
            # runs once an hour, which is fine.
            try:
                has_work = self._has_work_to_do(local_date)
            except Exception:
                # Don't let a transient DB blip silence the heartbeat.
                has_work = None
            log.info(
                "digest worker tick",
                extra={
                    "now_local": now_local.isoformat(),
                    "is_scheduled_time": scheduled,
                    "has_work_to_do": has_work,
                },
            )
            self._last_heartbeat_at = now_local

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
