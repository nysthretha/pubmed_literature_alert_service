from __future__ import annotations

import logging
import threading
import time

import httpx

EUTILS_BASE = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

log = logging.getLogger(__name__)


class _RateLimiter:
    """Sleeps until at least `min_interval` seconds have passed since the last wait()."""

    def __init__(self, min_interval: float) -> None:
        self.min_interval = min_interval
        self._last = 0.0
        self._lock = threading.Lock()

    def wait(self) -> None:
        with self._lock:
            now = time.monotonic()
            sleep_for = self._last + self.min_interval - now
            if sleep_for > 0:
                time.sleep(sleep_for)
            self._last = time.monotonic()


class EFetchClient:
    def __init__(self, tool: str, email: str, api_key: str | None):
        self.tool = tool
        self.email = email
        self.api_key = api_key
        self._limiter = _RateLimiter(min_interval=0.10 if api_key else 0.34)
        self._client = httpx.Client(timeout=30.0)

    def fetch(self, pmid: str) -> bytes:
        params = {
            "db": "pubmed",
            "id": pmid,
            "retmode": "xml",
            "tool": self.tool,
            "email": self.email,
        }
        if self.api_key:
            params["api_key"] = self.api_key

        last_err: Exception | None = None
        for attempt in range(3):
            self._limiter.wait()
            t0 = time.monotonic()
            try:
                resp = self._client.get(f"{EUTILS_BASE}/efetch.fcgi", params=params)
                duration_ms = int((time.monotonic() - t0) * 1000)
                if resp.status_code >= 500:
                    last_err = httpx.HTTPStatusError(
                        f"efetch status {resp.status_code}",
                        request=resp.request,
                        response=resp,
                    )
                else:
                    resp.raise_for_status()
                    log.info("efetch ok", extra={"pmid": pmid, "duration_ms": duration_ms, "attempt": attempt + 1})
                    return resp.content
            except httpx.HTTPError as e:
                last_err = e
            time.sleep(2 ** attempt)
        raise RuntimeError(f"efetch failed for pmid={pmid}: {last_err}")
