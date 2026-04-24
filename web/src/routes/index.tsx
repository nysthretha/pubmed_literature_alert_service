import { createFileRoute, redirect } from "@tanstack/react-router";

/**
 * Root "/" is not a rendered page — redirect to the first real destination.
 * If the user is authenticated, the `_auth` layout guard will wave them
 * through; otherwise the login route's redirect-if-auth logic kicks in.
 */
export const Route = createFileRoute("/")({
  beforeLoad: () => {
    throw redirect({ to: "/queries" });
  },
});
