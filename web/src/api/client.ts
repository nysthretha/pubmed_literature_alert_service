/**
 * API error mirroring the backend's envelope shape:
 *   { "error": { "code": "...", "message": "...", "fields": [...] } }
 * See docs/ARCHITECTURE.md on the server side.
 */
export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Array<{ field: string; message: string }>,
  ) {
    super(message);
    this.name = "APIError";
  }
}

interface ErrorEnvelope {
  error?: {
    code?: string;
    message?: string;
    fields?: Array<{ field: string; message: string }>;
  };
}

async function extractError(res: Response): Promise<APIError> {
  let parsed: ErrorEnvelope | null = null;
  try {
    parsed = await res.json();
  } catch {
    // non-JSON body; fall through with defaults
  }
  const body = parsed?.error ?? {};
  return new APIError(
    res.status,
    body.code ?? `http_${res.status}`,
    body.message ?? res.statusText ?? "request failed",
    body.fields,
  );
}

interface RequestOptions<TBody = unknown> {
  method?: string;
  body?: TBody;
  signal?: AbortSignal;
}

export async function apiRequest<TResponse, TBody = unknown>(
  path: string,
  { method = "GET", body, signal }: RequestOptions<TBody> = {},
): Promise<TResponse> {
  const init: RequestInit = {
    method,
    credentials: "include",
    signal,
    headers: {},
  };

  if (body !== undefined) {
    init.body = JSON.stringify(body);
    (init.headers as Record<string, string>)["Content-Type"] = "application/json";
  }

  const res = await fetch(path, init);

  if (!res.ok) {
    const err = await extractError(res);
    // Global 401 notifier: routes listen and invalidate the useAuth query,
    // which triggers the _auth guard's redirect to /login. Centralising this
    // means components don't need to each check for session expiry.
    if (res.status === 401) {
      window.dispatchEvent(new CustomEvent("auth:unauthorized"));
    }
    throw err;
  }

  // 204 No Content: nothing to parse. Caller should type TResponse as void.
  if (res.status === 204) {
    return undefined as TResponse;
  }

  const contentType = res.headers.get("Content-Type") ?? "";
  if (contentType.includes("application/json")) {
    return (await res.json()) as TResponse;
  }
  return (await res.text()) as unknown as TResponse;
}
