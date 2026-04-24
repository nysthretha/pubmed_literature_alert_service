import { apiRequest } from "./client";
import type { MatchedQuery } from "./articles";

export type DigestStatus = "pending" | "sent" | "failed";

export interface DigestSummary {
  id: number;
  sent_at: string;
  sent_local_date: string | null;
  status: DigestStatus;
  articles_included: number;
  manual: boolean;
}

export interface DigestArticle {
  pmid: string;
  title: string;
  journal: string | null;
  publication_date: string | null;
  matched_queries: MatchedQuery[];
}

export interface DigestDetail extends DigestSummary {
  error_message: string | null;
  articles: DigestArticle[];
}

export interface ListDigestsResponse {
  digests: DigestSummary[];
  total: number;
  has_more: boolean;
}

export async function listDigests(params: { limit?: number; offset?: number } = {}): Promise<ListDigestsResponse> {
  const q = new URLSearchParams();
  if (params.limit) q.set("limit", String(params.limit));
  if (params.offset) q.set("offset", String(params.offset));
  const qs = q.toString();
  return apiRequest<ListDigestsResponse>(`/api/digests${qs ? `?${qs}` : ""}`);
}

export async function getDigest(id: number): Promise<DigestDetail> {
  return apiRequest<DigestDetail>(`/api/digests/${id}`);
}

export async function triggerDigest(): Promise<void> {
  await apiRequest<{ status: string }>("/digest/trigger", { method: "POST" });
}
