import { createFileRoute, redirect } from "@tanstack/react-router";
import { AppShell } from "@/components/AppShell";
import { authQueryOptions } from "@/hooks/useAuth";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context, location }) => {
    const user = await context.queryClient.ensureQueryData(authQueryOptions);
    if (!user) {
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
      });
    }
    return { user };
  },
  component: AppShell,
});
