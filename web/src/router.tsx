import { createRouter } from "@tanstack/react-router";
import { QueryClient } from "@tanstack/react-query";
import { routeTree } from "./routeTree.gen";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: true,
      retry: (failureCount, error) => {
        // Don't retry 4xx — the caller's input is broken or unauthorized.
        if (
          typeof error === "object" &&
          error !== null &&
          "status" in error &&
          typeof (error as { status: unknown }).status === "number" &&
          (error as { status: number }).status < 500
        ) {
          return false;
        }
        return failureCount < 2;
      },
    },
  },
});

export const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0, // let TanStack Query own staleness
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
