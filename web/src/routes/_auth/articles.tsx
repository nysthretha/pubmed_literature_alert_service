import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export const Route = createFileRoute("/_auth/articles")({
  component: ArticlesPage,
});

function ArticlesPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Articles</h1>
        <p className="text-sm text-muted-foreground">
          Browse articles matched by your queries, with filters and search.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Coming in milestone 5d</CardTitle>
          <CardDescription>
            Paginated list with query filter, search, and detail view.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Backed by <code>GET /api/articles</code>.
        </CardContent>
      </Card>
    </div>
  );
}
