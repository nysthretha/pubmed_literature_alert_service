from __future__ import annotations

import logging
import smtplib
from abc import ABC, abstractmethod
from datetime import datetime
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from pathlib import Path

import httpx

log = logging.getLogger(__name__)


class DigestSender(ABC):
    @abstractmethod
    def send(self, *, subject: str, html: str, recipient: str, from_addr: str) -> None: ...


class FileSender(DigestSender):
    def __init__(self, output_dir: Path):
        self.output_dir = output_dir
        output_dir.mkdir(parents=True, exist_ok=True)

    def send(self, *, subject: str, html: str, recipient: str, from_addr: str) -> None:
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        path = self.output_dir / f"{ts}.html"
        path.write_text(html, encoding="utf-8")
        log.info(
            "preview written",
            extra={"path": str(path), "subject": subject, "recipient": recipient, "from": from_addr},
        )


class SMTPSender(DigestSender):
    def __init__(
        self,
        *,
        host: str,
        port: int,
        user: str | None,
        password: str | None,
        use_starttls: bool,
    ) -> None:
        self.host = host
        self.port = port
        self.user = user
        self.password = password
        self.use_starttls = use_starttls

    def send(self, *, subject: str, html: str, recipient: str, from_addr: str) -> None:
        msg = MIMEMultipart("alternative")
        msg["Subject"] = subject
        msg["From"] = from_addr
        msg["To"] = recipient
        msg.attach(MIMEText(html, "html", "utf-8"))

        with smtplib.SMTP(self.host, self.port, timeout=30) as smtp:
            smtp.ehlo()
            if self.use_starttls:
                smtp.starttls()
                smtp.ehlo()
            if self.user and self.password:
                smtp.login(self.user, self.password)
            smtp.send_message(msg)

        log.info(
            "smtp send ok",
            extra={"host": self.host, "port": self.port, "recipient": recipient, "starttls": self.use_starttls},
        )


class BrevoAPISender(DigestSender):
    """Posts the digest via Brevo's transactional email JSON API.

    Used when outbound SMTP is blocked (Railway Hobby plan blocks port 587).
    Brevo's documentation explicitly recommends the HTTPS API as the supported
    workaround: https://www.brevo.com/blog/transactional-email-api/

    The endpoint is the same Brevo API that powers their dashboard sends — no
    code paths through smtplib at all, so platform SMTP restrictions don't
    apply. Note that `sender.email` must correspond to a Brevo-verified sender
    (configured at https://app.brevo.com/senders/list); requests with an
    unverified sender are rejected with HTTP 400.

    htmlContent only — Brevo accepts htmlContent alone; textContent is
    optional. We skip plaintext to avoid the dependency burden of an HTML→text
    converter for what is, in practice, a single-recipient personal digest. If
    spam-folder issues surface, add textContent generation here.
    """

    BREVO_API_URL = "https://api.brevo.com/v3/smtp/email"

    def __init__(self, *, api_key: str, http_client: httpx.Client | None = None) -> None:
        self.api_key = api_key
        # http_client is injected for tests; real callers leave it None and we
        # construct a fresh client per send() with a 30s timeout, matching
        # SMTPSender's behavior.
        self._client = http_client

    def send(self, *, subject: str, html: str, recipient: str, from_addr: str) -> None:
        payload = {
            "sender": {"email": from_addr, "name": "PubMed Alerts"},
            "to": [{"email": recipient}],
            "subject": subject,
            "htmlContent": html,
        }
        headers = {
            "api-key": self.api_key,
            "accept": "application/json",
            "content-type": "application/json",
        }

        client = self._client or httpx.Client(timeout=30.0)
        try:
            resp = client.post(self.BREVO_API_URL, headers=headers, json=payload)
        finally:
            if self._client is None:
                client.close()

        if resp.status_code >= 300:
            log.error(
                "brevo api send failed",
                extra={
                    "status": resp.status_code,
                    "body": resp.text[:500],
                    "recipient": recipient,
                    "from": from_addr,
                },
            )
            resp.raise_for_status()

        message_id: str | None = None
        try:
            message_id = resp.json().get("messageId")
        except ValueError:
            pass  # 2xx with non-JSON body is unusual; log without message_id

        log.info(
            "brevo api send ok",
            extra={
                "status": resp.status_code,
                "recipient": recipient,
                "message_id": message_id,
            },
        )


def build_sender(
    mode: str,
    *,
    preview_dir: Path,
    smtp_host: str | None = None,
    smtp_port: int | None = None,
    smtp_user: str | None = None,
    smtp_password: str | None = None,
    mailpit_host: str = "mailpit",
    mailpit_port: int = 1025,
    brevo_api_key: str | None = None,
) -> DigestSender:
    if mode == "file":
        return FileSender(preview_dir)
    if mode == "mailpit":
        return SMTPSender(host=mailpit_host, port=mailpit_port, user=None, password=None, use_starttls=False)
    if mode == "smtp":
        if not smtp_host or not smtp_port:
            raise ValueError("DIGEST_MODE=smtp requires SMTP_HOST and SMTP_PORT")
        return SMTPSender(
            host=smtp_host, port=smtp_port, user=smtp_user, password=smtp_password, use_starttls=True,
        )
    if mode == "brevo_api":
        if not brevo_api_key:
            raise ValueError("DIGEST_MODE=brevo_api requires BREVO_API_KEY")
        return BrevoAPISender(api_key=brevo_api_key)
    raise ValueError(
        f"invalid DIGEST_MODE: {mode!r} (expected file, mailpit, smtp, or brevo_api)"
    )
