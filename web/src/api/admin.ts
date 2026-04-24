import { apiRequest } from "./client";
import type { User } from "./auth";

export interface AdminUser {
  id: number;
  username: string;
  email: string;
  is_admin: boolean;
  created_at: string;
  queries_count: number;
  last_login_at: string | null;
}

export interface ListUsersResponse {
  users: AdminUser[];
  total: number;
  has_more: boolean;
}

export async function listUsers(params: { limit?: number; offset?: number } = {}): Promise<ListUsersResponse> {
  const q = new URLSearchParams();
  if (params.limit) q.set("limit", String(params.limit));
  if (params.offset) q.set("offset", String(params.offset));
  const qs = q.toString();
  return apiRequest<ListUsersResponse>(`/api/admin/users${qs ? `?${qs}` : ""}`);
}

export interface CreateUserInput {
  username: string;
  email: string;
  password: string;
  is_admin?: boolean;
}

export async function createUser(input: CreateUserInput): Promise<User> {
  const { user } = await apiRequest<{ user: User }, CreateUserInput>("/api/admin/users", {
    method: "POST",
    body: input,
  });
  return user;
}

export interface UpdateUserInput {
  email?: string;
  is_admin?: boolean;
}

export async function updateUser(id: number, input: UpdateUserInput): Promise<User> {
  const { user } = await apiRequest<{ user: User }, UpdateUserInput>(`/api/admin/users/${id}`, {
    method: "PATCH",
    body: input,
  });
  return user;
}

export async function deleteUser(id: number): Promise<void> {
  await apiRequest<void>(`/api/admin/users/${id}`, { method: "DELETE" });
}

export async function resetUserPassword(id: number, newPassword: string): Promise<void> {
  await apiRequest<{ status: string }, { new_password: string }>(
    `/api/admin/users/${id}/reset-password`,
    { method: "POST", body: { new_password: newPassword } },
  );
}
