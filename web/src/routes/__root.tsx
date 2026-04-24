import { createRootRouteWithContext, Outlet } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { Button } from "@/components/ui/button";

interface RouterContext {
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
  errorComponent: ErrorComponent,
  notFoundComponent: NotFoundComponent,
});

function RootComponent() {
  return (
    <>
      <Outlet />
      <Toaster />
    </>
  );
}

function ErrorComponent({ error }: { error: Error }) {
  return (
    <div className="min-h-screen flex items-center justify-center p-6 bg-background text-foreground">
      <div className="max-w-md space-y-4 text-center">
        <h1 className="text-2xl font-semibold">Something went wrong</h1>
        <p className="text-sm text-muted-foreground break-words">{error.message}</p>
        <Button onClick={() => window.location.reload()}>Reload</Button>
      </div>
    </div>
  );
}

function NotFoundComponent() {
  return (
    <div className="min-h-screen flex items-center justify-center p-6 bg-background text-foreground">
      <div className="max-w-md space-y-4 text-center">
        <h1 className="text-2xl font-semibold">Not found</h1>
        <p className="text-sm text-muted-foreground">
          The page you're looking for doesn't exist.
        </p>
        <Button asChild variant="outline">
          <a href="/">Home</a>
        </Button>
      </div>
    </div>
  );
}
