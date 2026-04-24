import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export const Route = createFileRoute("/_auth/queries")({
  component: QueriesPage,
});

function QueriesPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Queries</h1>
        <p className="text-sm text-muted-foreground">
          Manage PubMed search queries, per-query filters, and poll intervals.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Coming in milestone 5d</CardTitle>
          <CardDescription>
            Query list, create/edit dialogs, and re-poll actions land here.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          The backend endpoints already exist — this placeholder becomes real
          UI once M5d adds the list/create/edit components.
        </CardContent>
      </Card>
    </div>
  );
}
