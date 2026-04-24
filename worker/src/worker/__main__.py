from __future__ import annotations

import logging
import os
import signal
import sys

from .consumer import Consumer
from .db import connect_db, wait_for_schema
from .logging_setup import setup_logging
from .pubmed_client import EFetchClient

log = logging.getLogger(__name__)


def main() -> int:
    setup_logging()

    rabbit_url = os.environ["RABBITMQ_URL"]
    pg_url = os.environ["POSTGRES_URL"]
    email = os.environ["PUBMED_EMAIL"]
    tool = os.environ.get("PUBMED_TOOL_NAME", "pubmed-alerts")
    api_key = os.environ.get("PUBMED_API_KEY") or None

    pg_conn = connect_db(pg_url)
    wait_for_schema(pg_conn)

    efetch = EFetchClient(tool=tool, email=email, api_key=api_key)
    consumer = Consumer(rabbit_url=rabbit_url, pg_conn=pg_conn, efetch=efetch)

    def _shutdown(signum, _frame):
        log.info("shutdown signal", extra={"signal": signum})
        consumer.stop()

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    try:
        consumer.run()
    finally:
        try:
            pg_conn.close()
        except Exception:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main())
