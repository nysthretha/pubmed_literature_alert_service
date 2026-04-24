import { apiRequest } from "./client";

export interface User {
  id: number;
  username: string;
  email: string;
  is_admin: boolean;
}

export interface LoginInput {
  username: string;
  password: string;
}

export async function login(input: LoginInput): Promise<User> {
  const { user } = await apiRequest<{ user: User }, LoginInput>("/api/auth/login", {
    method: "POST",
    body: input,
  });
  return user;
}

export async function logout(): Promise<void> {
  await apiRequest<void>("/api/auth/logout", { method: "POST" });
}

export async function getCurrentUser(): Promise<User> {
  const { user } = await apiRequest<{ user: User }>("/api/auth/me");
  return user;
}
