import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/shared/PageHeader";
import { EmptyState } from "@/components/shared/EmptyState";
import { ErrorState } from "@/components/shared/ErrorState";
import { listArticles, type Article } from "@/api/articles";
import { listQueries } from "@/api/queries";
import { APIError } from "@/api/client";
import { ArticleTable } from "./-components/ArticleTable";
import { ArticleDetailDrawer } from "./-components/ArticleDetailDrawer";
import { ArticleFilters, sinceParam, type Filters } from "./-components/ArticleFilters";

const PAGE_SIZE = 50;

export const Route = createFileRoute("/_auth/articles/")({
  component: ArticlesPage,
});

function ArticlesPage() {
  const [filters, setFilters] = useState<Filters>({
    search: "",
    queryId: null,
    sinceDays: null,
  });
  const [selected, setSelected] = useState<Article | null>(null);

  const queriesQuery = useQuery({ queryKey: ["queries"], queryFn: listQueries });

  const articlesQuery = useInfiniteQuery({
    queryKey: [
      "articles",
      filters.search,
      filters.queryId,
      filters.sinceDays,
    ],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      listArticles({
        limit: PAGE_SIZE,
        offset: pageParam as number,
        search: filters.search || undefined,
        query_id: filters.queryId ?? undefined,
        since: sinceParam(filters.sinceDays),
      }),
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.has_more) return undefined;
      return allPages.reduce((sum, p) => sum + p.articles.length, 0);
    },
  });

  const pages = articlesQuery.data?.pages ?? [];
  const articles = pages.flatMap((p) => p.articles);
  const total = pages[0]?.total ?? 0;

  const hasNoQueries =
    queriesQuery.isSuccess && (queriesQuery.data?.length ?? 0) === 0;
  const filtersActive =
    filters.search.length > 0 || filters.queryId !== null || filters.sinceDays !== null;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Articles"
        description={
          total > 0
            ? `${total} article${total === 1 ? "" : "s"} across your queries.`
            : "Articles matched by your queries appear here."
        }
      />

      {hasNoQueries ? (
        <EmptyState
          title="No queries yet"
          description="Articles land here after the scheduler polls your queries. Create a query first."
          action={
            <Button asChild>
              <Link to="/queries">Go to Queries</Link>
            </Button>
          }
        />
      ) : (
        <>
          <ArticleFilters
            filters={filters}
            onChange={setFilters}
            queries={queriesQuery.data ?? []}
          />

          {articlesQuery.isLoading && (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-20" />
              ))}
            </div>
          )}

          {articlesQuery.error && (
            <ErrorState
              message={
                articlesQuery.error instanceof APIError
                  ? articlesQuery.error.message
                  : String(articlesQuery.error)
              }
              onRetry={() => articlesQuery.refetch()}
            />
          )}

          {articles.length === 0 && !articlesQuery.isLoading && !articlesQuery.error && (
            <EmptyState
              title={filtersActive ? "No articles match these filters" : "No articles yet"}
              description={
                filtersActive
                  ? "Try clearing filters or broadening the search."
                  : "Articles will appear after your queries poll PubMed (every few hours by default)."
              }
            />
          )}

          {articles.length > 0 && (
            <>
              <ArticleTable articles={articles} onRowClick={setSelected} />
              {articlesQuery.hasNextPage && (
                <div className="flex justify-center">
                  <Button
                    variant="outline"
                    onClick={() => articlesQuery.fetchNextPage()}
                    disabled={articlesQuery.isFetchingNextPage}
                  >
                    {articlesQuery.isFetchingNextPage ? "Loading…" : "Load more"}
                  </Button>
                </div>
              )}
            </>
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
