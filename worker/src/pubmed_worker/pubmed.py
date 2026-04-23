from __future__ import annotations

import time
from dataclasses import dataclass
from datetime import date

import httpx
from lxml import etree

from .logging_setup import get_logger
from .ratelimit import RateLimiter

EUTILS_BASE = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

log = get_logger(__name__)


@dataclass
class Author:
    last_name: str | None = None
    fore_name: str | None = None
    initials: str | None = None
    collective_name: str | None = None


@dataclass
class Article:
    pmid: str
    title: str
    abstract: str | None
    journal: str | None
    publication_date: date | None
    authors: list[Author]
    publication_types: list[str]


class EFetchClient:
    def __init__(self, tool: str, email: str, api_key: str | None):
        self.tool = tool
        self.email = email
        self.api_key = api_key
        self.limiter = RateLimiter(min_interval=0.10 if api_key else 0.34)
        self.client = httpx.Client(timeout=30.0)

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
            self.limiter.wait()
            t0 = time.monotonic()
            try:
                resp = self.client.get(f"{EUTILS_BASE}/efetch.fcgi", params=params)
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


def parse_pubmed_xml(xml_bytes: bytes, pmid: str) -> Article:
    root = etree.fromstring(xml_bytes)
    article_elem = root.find(".//PubmedArticle")
    if article_elem is None:
        raise ValueError(f"no PubmedArticle in response for pmid={pmid}")

    title_elem = article_elem.find(".//Article/ArticleTitle")
    title = _full_text(title_elem) if title_elem is not None else ""

    return Article(
        pmid=pmid,
        title=title,
        abstract=_extract_abstract(article_elem),
        journal=_text(article_elem.find(".//Article/Journal/Title")),
        publication_date=_extract_pub_date(article_elem),
        authors=_extract_authors(article_elem),
        publication_types=_extract_publication_types(article_elem),
    )


def _text(elem) -> str | None:
    return elem.text if elem is not None and elem.text else None


def _full_text(elem) -> str:
    return "".join(elem.itertext()).strip() if elem is not None else ""


def _extract_abstract(article_elem) -> str | None:
    parts: list[str] = []
    for abst in article_elem.findall(".//Article/Abstract/AbstractText"):
        text = _full_text(abst)
        if not text:
            continue
        label = abst.get("Label")
        parts.append(f"{label}: {text}" if label else text)
    return "\n\n".join(parts) if parts else None


_MONTHS = {
    "jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
    "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}


def _extract_pub_date(article_elem) -> date | None:
    pd = article_elem.find(".//Article/Journal/JournalIssue/PubDate")
    if pd is None:
        return None

    year_text = _text(pd.find("Year"))
    if year_text:
        try:
            year = int(year_text)
        except ValueError:
            return None
        month_text = _text(pd.find("Month"))
        day_text = _text(pd.find("Day"))
        month = 1
        day = 1
        if month_text:
            m = month_text.strip().lower()
            month = int(m) if m.isdigit() else _MONTHS.get(m[:3], 1)
        if day_text and day_text.isdigit():
            day = int(day_text)
        try:
            return date(year, month, day)
        except ValueError:
            return None

    medline = _text(pd.find("MedlineDate"))
    if medline:
        token = medline.strip().split()[0]
        if token.isdigit() and len(token) == 4:
            return date(int(token), 1, 1)
    return None


def _extract_authors(article_elem) -> list[Author]:
    out: list[Author] = []
    for a in article_elem.findall(".//Article/AuthorList/Author"):
        out.append(Author(
            last_name=_text(a.find("LastName")),
            fore_name=_text(a.find("ForeName")),
            initials=_text(a.find("Initials")),
            collective_name=_text(a.find("CollectiveName")),
        ))
    return out


def _extract_publication_types(article_elem) -> list[str]:
    out: list[str] = []
    for pt in article_elem.findall(".//Article/PublicationTypeList/PublicationType"):
        t = _text(pt)
        if t:
            out.append(t)
    return out
