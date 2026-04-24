import { apiRequest } from "./client";

export interface Query {
  id: number;
  name: string;
  query_string: string;
  last_polled_at: string | null;
  poll_interval_seconds: number;
  is_active: boolean;
  min_abstract_length: number;
  publication_type_allowlist: string[] | null;
  publication_type_blocklist: string[];
  notes: string | null;
  created_at: string;
  article_count: number;
}

export interface CreateQueryInput {
  name: string;
  query_string: string;
  poll_interval_seconds?: number;
  is_active?: boolean;
  min_abstract_length?: number;
  publication_type_allowlist?: string[] | null;
  publication_type_blocklist?: string[];
  notes?: string | null;
}

export type UpdateQueryInput = Partial<CreateQueryInput>;

export async function listQueries(): Promise<Query[]> {
  const { queries } = await apiRequest<{ queries: Query[] }>("/api/queries");
  return queries ?? [];
}

export async function getQuery(id: number): Promise<Query> {
  return apiRequest<Query>(`/api/queries/${id}`);
}

export async function createQuery(input: CreateQueryInput): Promise<Query> {
  return apiRequest<Query, CreateQueryInput>("/api/queries", {
    method: "POST",
    body: input,
  });
}

export async function updateQuery(id: number, input: UpdateQueryInput): Promise<Query> {
  return apiRequest<Query, UpdateQueryInput>(`/api/queries/${id}`, {
    method: "PATCH",
    body: input,
  });
}

export async function deleteQuery(id: number): Promise<void> {
  await apiRequest<void>(`/api/queries/${id}`, { method: "DELETE" });
}

export async function repollQuery(id: number): Promise<void> {
  await apiRequest<{ status: string }>(`/api/queries/${id}/repoll`, { method: "POST" });
}
