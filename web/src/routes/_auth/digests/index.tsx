import { createFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/shared/PageHeader";
import { EmptyState } from "@/components/shared/EmptyState";
import { ErrorState } from "@/components/shared/ErrorState";
import { listDigests } from "@/api/digests";
import { APIError } from "@/api/client";
import { DigestCard } from "./-components/DigestCard";
import { SendTestDigestButton } from "./-components/SendTestDigestButton";

export const Route = createFileRoute("/_auth/digests/")({
  component: DigestsPage,
});

function DigestsPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["digests"],
    queryFn: () => listDigests({ limit: 50 }),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="Digests"
        description="History of digest emails sent (or attempted)."
        actions={<SendTestDigestButton />}
      />

      {isLoading && (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-20" />
          ))}
        </div>
      )}

      {error && (
        <ErrorState
          message={error instanceof APIError ? error.message : String(error)}
          onRetry={() => refetch()}
        />
      )}

      {data && data.digests.length === 0 && !isLoading && (
        <EmptyState
          title="No digests yet"
          description="Your first digest goes out at the scheduled time once you have matched articles. You can trigger one manually from the button above."
        />
      )}

      {data && data.digests.length > 0 && (
        <div className="space-y-3">
          {data.digests.map((d) => (
            <DigestCard key={d.id} digest={d} />
          ))}
        </div>
      )}
    </div>
  );
}
