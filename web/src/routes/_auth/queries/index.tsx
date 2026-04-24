import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/shared/PageHeader";
import { EmptyState } from "@/components/shared/EmptyState";
import { ErrorState } from "@/components/shared/ErrorState";
import { listQueries, type Query } from "@/api/queries";
import { APIError } from "@/api/client";
import { QueryCard } from "./-components/QueryCard";
import { QueryFormDialog } from "./-components/QueryFormDialog";
import { DeleteQueryDialog } from "./-components/DeleteQueryDialog";

export const Route = createFileRoute("/_auth/queries/")({
  component: QueriesPage,
});

type DialogState =
  | { type: "closed" }
  | { type: "create" }
  | { type: "edit"; query: Query }
  | { type: "delete"; query: Query };

function QueriesPage() {
  const [dialog, setDialog] = useState<DialogState>({ type: "closed" });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["queries"],
    queryFn: listQueries,
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="Queries"
        description="PubMed search queries you want alerts for."
        actions={
          <Button onClick={() => setDialog({ type: "create" })}>
            <Plus />
            New query
          </Button>
        }
      />

      {isLoading && (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-52" />
          ))}
        </div>
      )}

      {error && (
        <ErrorState
          message={error instanceof APIError ? error.message : String(error)}
          onRetry={() => refetch()}
        />
      )}

      {data && data.length === 0 && !isLoading && (
        <EmptyState
          title="No queries yet"
          description="Create a query to start getting PubMed alerts. Each query polls the E-utilities on its own interval."
          action={
            <Button onClick={() => setDialog({ type: "create" })}>
              <Plus />
              New query
            </Button>
          }
        />
      )}

      {data && data.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {data.map((q) => (
            <QueryCard
              key={q.id}
              query={q}
              onEdit={(query) => setDialog({ type: "edit", query })}
              onDelete={(query) => setDialog({ type: "delete", query })}
            />
          ))}
        </div>
      )}

      <QueryFormDialog
        open={dialog.type === "create" || dialog.type === "edit"}
        onOpenChange={(open) => {
          if (!open) setDialog({ type: "closed" });
        }}
        mode={dialog.type === "edit" ? "edit" : "create"}
        query={dialog.type === "edit" ? dialog.query : undefined}
      />
      <DeleteQueryDialog
        open={dialog.type === "delete"}
        onOpenChange={(open) => {
          if (!open) setDialog({ type: "closed" });
        }}
        query={dialog.type === "delete" ? dialog.query : null}
      />
    </div>
  );
}
