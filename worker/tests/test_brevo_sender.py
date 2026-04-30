"""Tests for BrevoAPISender — verifies request shape against a mocked transport.

The pattern: construct an httpx.Client backed by httpx.MockTransport, inject it
into BrevoAPISender via the ``http_client`` constructor parameter, capture the
synthesized request, and assert on its URL, headers, and JSON body.

No live HTTP calls. Run with: ``cd worker && pip install '.[test]' && pytest``.
"""

from __future__ import annotations

import json

import httpx
import pytest

from worker.digest.sender import BrevoAPISender


def _client_with_handler(handler):
    """Wrap a request handler in a MockTransport-backed httpx.Client."""
    transport = httpx.MockTransport(handler)
    return httpx.Client(transport=transport)


def test_request_targets_brevo_endpoint_with_correct_headers():
    captured: dict[str, object] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        captured["method"] = request.method
        captured["headers"] = dict(request.headers)
        captured["body"] = json.loads(request.content)
        return httpx.Response(201, json={"messageId": "<abc@brevo>"})

    sender = BrevoAPISender(api_key="test-key", http_client=_client_with_handler(handler))
    sender.send(
        subject="Test digest",
        html="<p>hello</p>",
        recipient="recipient@example.com",
        from_addr="sender@example.com",
    )

    assert captured["url"] == BrevoAPISender.BREVO_API_URL
    assert captured["method"] == "POST"
    assert captured["headers"]["api-key"] == "test-key"
    assert captured["headers"]["accept"] == "application/json"
    assert captured["headers"]["content-type"] == "application/json"


def test_request_body_matches_brevo_schema():
    captured: dict[str, object] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = json.loads(request.content)
        return httpx.Response(201, json={"messageId": "<id>"})

    sender = BrevoAPISender(api_key="k", http_client=_client_with_handler(handler))
    sender.send(
        subject="Subject line",
        html="<h1>HTML body</h1>",
        recipient="to@example.com",
        from_addr="from@example.com",
    )

    body = captured["body"]
    assert body["sender"] == {"email": "from@example.com", "name": "PubMed Alerts"}
    assert body["to"] == [{"email": "to@example.com"}]
    assert body["subject"] == "Subject line"
    assert body["htmlContent"] == "<h1>HTML body</h1>"


def test_text_content_is_omitted():
    """We deliberately don't generate plaintext — Brevo accepts htmlContent
    alone. Lock that decision in: the request body must NOT include
    textContent. If a future change adds it, this test will fail and force a
    deliberate code review."""
    captured: dict[str, object] = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["body"] = json.loads(request.content)
        return httpx.Response(201, json={"messageId": "<id>"})

    sender = BrevoAPISender(api_key="k", http_client=_client_with_handler(handler))
    sender.send(subject="s", html="<p>h</p>", recipient="a@b.com", from_addr="c@d.com")

    assert "textContent" not in captured["body"]


def test_non_2xx_response_raises_http_error():
    def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            400,
            json={"code": "missing_parameter", "message": "sender.email is invalid"},
        )

    sender = BrevoAPISender(api_key="k", http_client=_client_with_handler(handler))

    with pytest.raises(httpx.HTTPStatusError):
        sender.send(subject="s", html="<p>h</p>", recipient="a@b.com", from_addr="c@d.com")


def test_logs_structured_failure_context_on_4xx(caplog):
    """The failure-log fields are part of the runbook surface — operators
    triage Brevo errors via the structured log line, so the keys are as much a
    contract as the request body."""
    def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            401,
            json={"code": "unauthorized", "message": "Key not found"},
        )

    sender = BrevoAPISender(api_key="bad", http_client=_client_with_handler(handler))

    with caplog.at_level("ERROR", logger="worker.digest.sender"):
        with pytest.raises(httpx.HTTPStatusError):
            sender.send(
                subject="s", html="<p>h</p>",
                recipient="a@b.com", from_addr="c@d.com",
            )

    err = next((r for r in caplog.records if r.message == "brevo api send failed"), None)
    assert err is not None, "expected 'brevo api send failed' log record"
    assert err.status == 401
    assert err.recipient == "a@b.com"
    assert "Key not found" in err.body


def test_factory_requires_brevo_api_key():
    from pathlib import Path

    from worker.digest.sender import build_sender

    with pytest.raises(ValueError, match="BREVO_API_KEY"):
        build_sender("brevo_api", preview_dir=Path("/tmp"), brevo_api_key=None)


def test_factory_constructs_brevo_sender():
    from pathlib import Path

    from worker.digest.sender import build_sender

    sender = build_sender("brevo_api", preview_dir=Path("/tmp"), brevo_api_key="k")
    assert isinstance(sender, BrevoAPISender)
