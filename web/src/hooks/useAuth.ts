import { queryOptions, useQuery } from "@tanstack/react-query";
import { APIError } from "@/api/client";
import { getCurrentUser, type User } from "@/api/auth";

/**
 * Shared query options for the current-user lookup. `null` means
 * authenticated-as-nobody (the server returned 401); anything else throws.
 * Routes use `queryClient.ensureQueryData(authQueryOptions)` inside
 * `beforeLoad` to gate access.
 */
export const authQueryOptions = queryOptions<User | null>({
  queryKey: ["auth", "me"],
  queryFn: async () => {
    try {
      return await getCurrentUser();
    } catch (err) {
      if (err instanceof APIError && err.status === 401) {
        return null;
      }
      throw err;
    }
  },
  staleTime: 5 * 60 * 1000, // 5 min — cookies last 30 days, no need to re-check aggressively
  retry: false,
});

export function useAuth() {
  return useQuery(authQueryOptions);
}
