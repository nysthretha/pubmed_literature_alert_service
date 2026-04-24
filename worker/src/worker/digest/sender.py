from __future__ import annotations

import logging
import smtplib
from abc import ABC, abstractmethod
from datetime import datetime
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from pathlib import Path

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
    raise ValueError(f"invalid DIGEST_MODE: {mode!r} (expected file, mailpit, or smtp)")
