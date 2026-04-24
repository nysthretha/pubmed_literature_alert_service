import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export const Route = createFileRoute("/_auth/digests")({
  component: DigestsPage,
});

function DigestsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Digests</h1>
        <p className="text-sm text-muted-foreground">
          History of daily digests sent (or attempted).
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Coming in milestone 5d</CardTitle>
          <CardDescription>Digest list + detail view with article lineup.</CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Backed by <code>GET /api/digests</code> and <code>GET /api/digests/:id</code>.
        </CardContent>
      </Card>
    </div>
  );
}
