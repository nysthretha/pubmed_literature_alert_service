import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";
import { PageHeader } from "@/components/shared/PageHeader";
import { ErrorState } from "@/components/shared/ErrorState";
import { ArticleDetailDrawer } from "@/routes/_auth/articles/-components/ArticleDetailDrawer";
import { getDigest, type DigestArticle } from "@/api/digests";
import { getArticle, type Article } from "@/api/articles";
import { APIError } from "@/api/client";
import { formatDate, relativeTime } from "@/lib/format";

export const Route = createFileRoute("/_auth/digests/$id")({
  component: DigestDetailPage,
});

function DigestDetailPage() {
  const { id } = Route.useParams();
  const [selected, setSelected] = useState<Article | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["digest", id],
    queryFn: () => getDigest(Number(id)),
  });

  const openArticle = async (da: DigestArticle) => {
    try {
      const full = await getArticle(da.pmid);
      setSelected(full);
    } catch (e) {
      // Defensive: if the user's matching was deleted since this digest was
      // built, /api/articles/:pmid returns 404. Show a minimal placeholder so
      // the drawer still opens with what we have.
      if (e instanceof APIError && e.status === 404) {
        setSelected({
          pmid: da.pmid,
          title: da.title,
          abstract: null,
          journal: da.journal,
          publication_date: da.publication_date,
          authors: [],
          publication_types: [],
          fetched_at: new Date().toISOString(),
          matched_queries: da.matched_queries,
        });
        return;
      }
      throw e;
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <Button asChild variant="ghost" size="sm" className="-ml-2 mb-2">
          <Link to="/digests">
            <ArrowLeft />
            All digests
          </Link>
        </Button>

        <PageHeader
          title={
            data
              ? `Digest · ${formatDate(data.sent_local_date ?? data.sent_at)}`
              : "Digest"
          }
          description={
            data
              ? `${data.articles_included} article${
                  data.articles_included === 1 ? "" : "s"
                } · ${relativeTime(data.sent_at)}`
              : undefined
          }
        />
      </div>

      {isLoading && (
        <div className="space-y-3">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      )}

      {error && (
        <ErrorState
          message={error instanceof APIError ? error.message : String(error)}
          onRetry={() => refetch()}
        />
      )}

      {data && (
        <>
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                Delivery
                {data.status === "sent" && <Badge variant="success">Sent</Badge>}
                {data.status === "failed" && <Badge variant="destructive">Failed</Badge>}
                {data.status === "pending" && <Badge variant="warning">Pending</Badge>}
                {data.manual && <Badge variant="outline">Manual</Badge>}
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm space-y-1">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Sent at</span>
                <span>{new Date(data.sent_at).toLocaleString()}</span>
              </div>
              {data.error_message && (
                <>
                  <Separator className="my-2" />
                  <div>
                    <div className="text-muted-foreground">Error</div>
                    <pre className="mt-1 whitespace-pre-wrap break-words text-xs text-destructive">
                      {data.error_message}
                    </pre>
                  </div>
                </>
              )}
            </CardContent>
          </Card>

          {data.articles.length > 0 ? (
            <div className="rounded-md border">
              <Table>
                <TableBody>
                  {data.articles.map((a) => (
                    <TableRow
                      key={a.pmid}
                      onClick={() => openArticle(a)}
                      className="cursor-pointer"
                    >
                      <TableCell className="py-3">
                        <div className="space-y-1">
                          <div className="font-medium leading-snug line-clamp-2">
                            {a.title}
                          </div>
                          <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
                            {a.journal && <span className="italic">{a.journal}</span>}
                            {a.publication_date && (
                              <>
                                <span>·</span>
                                <span>{formatDate(a.publication_date)}</span>
                              </>
                            )}
                            {a.matched_queries.map((q) => (
                              <Badge key={q.id} variant="secondary" className="font-normal">
                                {q.name}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              This digest contained no articles.
            </p>
          )}
        </>
      )}

      <ArticleDetailDrawer
        article={selected}
        open={selected !== null}
        onOpenChange={(o) => !o && setSelected(null)}
      />
    </div>
  );
}
