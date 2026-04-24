import { apiRequest } from "./client";

export interface MatchedQuery {
  id: number;
  name: string;
}

export interface Author {
  last_name?: string | null;
  fore_name?: string | null;
  initials?: string | null;
  collective_name?: string | null;
}

export interface Article {
  pmid: string;
  title: string;
  abstract: string | null;
  journal: string | null;
  publication_date: string | null; // YYYY-MM-DD
  authors: Author[];
  publication_types: string[];
  fetched_at: string;
  matched_queries: MatchedQuery[];
}

export interface ListArticlesResponse {
  articles: Article[];
  total: number;
  has_more: boolean;
}

export interface ListArticlesParams {
  limit?: number;
  offset?: number;
  query_id?: number;
  since?: string; // RFC3339
  search?: string;
}

export async function listArticles(params: ListArticlesParams = {}): Promise<ListArticlesResponse> {
  const q = new URLSearchParams();
  if (params.limit) q.set("limit", String(params.limit));
  if (params.offset) q.set("offset", String(params.offset));
  if (params.query_id) q.set("query_id", String(params.query_id));
  if (params.since) q.set("since", params.since);
  if (params.search) q.set("search", params.search);
  const qs = q.toString();
  return apiRequest<ListArticlesResponse>(`/api/articles${qs ? `?${qs}` : ""}`);
}

export async function getArticle(pmid: string): Promise<Article> {
  return apiRequest<Article>(`/api/articles/${encodeURIComponent(pmid)}`);
}

export function authorsSummary(authors: Author[] | null | undefined, max = 3): string {
  if (!authors || authors.length === 0) return "(no authors)";
  const parts: string[] = [];
  for (const a of authors.slice(0, max)) {
    if (a.collective_name) {
      parts.push(a.collective_name);
      continue;
    }
    const last = a.last_name ?? "";
    const ini = a.initials ?? "";
    if (last && ini) parts.push(`${last} ${ini}`);
    else if (last) parts.push(last);
  }
  const joined = parts.join(", ");
  return authors.length > max ? `${joined} et al.` : joined;
}
