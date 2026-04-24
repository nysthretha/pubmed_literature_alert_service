import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ChevronDown,
  ChevronUp,
  Pencil,
  RefreshCcw,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { repollQuery, type Query } from "@/api/queries";
import { APIError } from "@/api/client";
import { formatInterval, relativeTime } from "@/lib/format";

interface Props {
  query: Query;
  onEdit: (q: Query) => void;
  onDelete: (q: Query) => void;
}

export function QueryCard({ query, onEdit, onDelete }: Props) {
  const [expanded, setExpanded] = useState(false);
  const queryClient = useQueryClient();

  const repollMutation = useMutation({
    mutationFn: () => repollQuery(query.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["queries"] });
    },
  });

  const onRepollClick = () => {
    toast.promise(repollMutation.mutateAsync(), {
      loading: `Queuing re-poll for "${query.name}"…`,
      success: `Re-poll queued. Scheduler will fetch on its next tick.`,
      error: (err: unknown) =>
        err instanceof APIError ? err.message : "Re-poll failed",
    });
  };

  return (
    <Card className="flex flex-col">
      <CardContent className="flex flex-col gap-3 p-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex items-center gap-2">
              <h3 className="truncate font-semibold">{query.name}</h3>
              {!query.is_active && (
                <Badge variant="outline" className="text-muted-foreground">
                  Inactive
                </Badge>
              )}
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <p className="line-clamp-2 cursor-help font-mono text-xs text-muted-foreground">
                  {query.query_string}
                </p>
              </TooltipTrigger>
              <TooltipContent className="max-w-md whitespace-pre-wrap font-mono text-xs">
                {query.query_string}
              </TooltipContent>
            </Tooltip>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>
            <span className="font-medium text-foreground">{query.article_count}</span>{" "}
            {query.article_count === 1 ? "article" : "articles"}
          </span>
          <span>
            polled {relativeTime(query.last_polled_at)}
          </span>
          <span>every {formatInterval(query.poll_interval_seconds)}</span>
        </div>

        {expanded && (
          <div className="space-y-2 rounded-md border bg-muted/30 p-3 text-xs">
            <MetaRow label="Min abstract length">
              {query.min_abstract_length > 0
                ? `${query.min_abstract_length} chars`
                : "no minimum"}
            </MetaRow>
            <MetaRow label="Allowlist">
              {query.publication_type_allowlist?.length
                ? query.publication_type_allowlist.join(", ")
                : "all types allowed"}
            </MetaRow>
            <MetaRow label="Blocklist">
              {query.publication_type_blocklist.length
                ? query.publication_type_blocklist.join(", ")
                : "none"}
            </MetaRow>
            {query.notes && <MetaRow label="Notes">{query.notes}</MetaRow>}
          </div>
        )}

        <div className="flex items-center justify-between">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setExpanded((s) => !s)}
            className="-ml-2 text-muted-foreground"
          >
            {expanded ? (
              <>
                <ChevronUp />
                Less
              </>
            ) : (
              <>
                <ChevronDown />
                More
              </>
            )}
          </Button>
          <div className="flex items-center gap-1">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={onRepollClick}
                  aria-label="Re-poll"
                  disabled={repollMutation.isPending}
                >
                  <RefreshCcw />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Re-poll (clear last_polled_at)</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onEdit(query)}
                  aria-label="Edit"
                >
                  <Pencil />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Edit</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onDelete(query)}
                  aria-label="Delete"
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Delete</TooltipContent>
            </Tooltip>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function MetaRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline gap-2">
      <span className="w-24 shrink-0 text-muted-foreground">{label}</span>
      <span className="flex-1 break-words">{children}</span>
    </div>
  );
}
