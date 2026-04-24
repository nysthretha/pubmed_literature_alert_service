import { Link } from "@tanstack/react-router";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import type { DigestSummary } from "@/api/digests";
import { formatDate, relativeTime } from "@/lib/format";

interface Props {
  digest: DigestSummary;
}

export function DigestCard({ digest }: Props) {
  return (
    <Link
      to="/digests/$id"
      params={{ id: String(digest.id) }}
      className="block no-underline"
    >
      <Card className="transition-colors hover:bg-accent/40">
        <CardContent className="flex items-start justify-between gap-4 p-5">
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold">
                {formatDate(digest.sent_local_date ?? digest.sent_at)}
              </h3>
              <StatusBadge status={digest.status} />
              {digest.manual && <Badge variant="outline">Manual</Badge>}
            </div>
            <p className="text-xs text-muted-foreground">
              {digest.articles_included}{" "}
              {digest.articles_included === 1 ? "article" : "articles"} included
              · {relativeTime(digest.sent_at)}
            </p>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}

function StatusBadge({ status }: { status: DigestSummary["status"] }) {
  if (status === "sent") return <Badge variant="success">Sent</Badge>;
  if (status === "failed") return <Badge variant="destructive">Failed</Badge>;
  return <Badge variant="warning">Pending</Badge>;
}
