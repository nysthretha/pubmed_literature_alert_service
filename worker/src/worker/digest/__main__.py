from __future__ import annotations

import logging
import os
import sys
from pathlib import Path
from zoneinfo import ZoneInfo

from ..db import connect_db, wait_for_schema
from ..logging_setup import setup_logging
from .scheduler import DigestWorker
from .sender import build_sender

log = logging.getLogger(__name__)

PREVIEW_DIR = Path("/app/digest-preview")


def main() -> int:
    setup_logging()

    rabbit_url = os.environ["RABBITMQ_URL"]
    pg_url = os.environ["POSTGRES_URL"]
    recipient = os.environ["DIGEST_RECIPIENT"]
    from_addr = os.environ.get("SMTP_FROM") or recipient
    tz_name = os.environ.get("DIGEST_TIMEZONE", "UTC")
    send_hour = int(os.environ.get("DIGEST_SEND_HOUR", "7"))
    subject_prefix = os.environ.get("DIGEST_SUBJECT_PREFIX", "PubMed Alerts")
    mode = os.environ.get("DIGEST_MODE", "file").lower()

    tz = ZoneInfo(tz_name)

    sender = build_sender(
        mode,
        preview_dir=PREVIEW_DIR,
        smtp_host=os.environ.get("SMTP_HOST"),
        smtp_port=int(os.environ["SMTP_PORT"]) if os.environ.get("SMTP_PORT") else None,
        smtp_user=os.environ.get("SMTP_USER") or None,
        smtp_password=os.environ.get("SMTP_PASSWORD") or None,
        brevo_api_key=os.environ.get("BREVO_API_KEY") or None,
    )

    pg_conn = connect_db(pg_url)
    wait_for_schema(pg_conn)

    worker = DigestWorker(
        rabbit_url=rabbit_url,
        pg_conn=pg_conn,
        sender=sender,
        tz=tz,
        send_hour=send_hour,
        recipient=recipient,
        from_addr=from_addr,
        subject_prefix=subject_prefix,
        record_in_db=(mode != "file"),
    )

    try:
        worker.run()
    finally:
        try:
            pg_conn.close()
        except Exception:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main())
