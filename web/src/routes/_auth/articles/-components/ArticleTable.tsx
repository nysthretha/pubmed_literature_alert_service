import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { type Article, authorsSummary } from "@/api/articles";
import { formatDate, relativeTime } from "@/lib/format";
import { abstractSnippet } from "@/lib/text";

interface Props {
  articles: Article[];
  onRowClick: (a: Article) => void;
}

export function ArticleTable({ articles, onRowClick }: Props) {
  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[55%]">Title</TableHead>
            <TableHead className="w-[20%]">Journal</TableHead>
            <TableHead className="w-[15%]">Published</TableHead>
            <TableHead className="w-[10%] text-right">Fetched</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {articles.map((a) => (
            <TableRow
              key={a.pmid}
              onClick={() => onRowClick(a)}
              className="cursor-pointer"
            >
              <TableCell className="py-3">
                <div className="space-y-1">
                  <div className="font-medium leading-snug line-clamp-2">{a.title}</div>
                  {a.abstract && (
                    <div className="text-xs text-muted-foreground line-clamp-2 leading-snug">
                      {abstractSnippet(a.abstract, 180)}
                    </div>
                  )}
                  <div className="flex flex-wrap items-center gap-1 pt-0.5 text-xs text-muted-foreground">
                    <span>{authorsSummary(a.authors)}</span>
                    {a.matched_queries.length > 0 && <span>·</span>}
                    {a.matched_queries.map((q) => (
                      <Badge key={q.id} variant="secondary" className="font-normal">
                        {q.name}
                      </Badge>
                    ))}
                  </div>
                </div>
              </TableCell>
              <TableCell className="text-sm text-muted-foreground italic">
                {a.journal ?? "—"}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground">
                {formatDate(a.publication_date)}
              </TableCell>
              <TableCell className="text-right text-xs text-muted-foreground">
                {relativeTime(a.fetched_at)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
