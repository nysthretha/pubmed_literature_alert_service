import { useState } from "react";
import { ExternalLink, Link as LinkIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { authorsSummary, type Article } from "@/api/articles";
import { formatDate, relativeTime } from "@/lib/format";

interface Props {
  article: Article | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ArticleDetailDrawer({ article, open, onOpenChange }: Props) {
  const [copied, setCopied] = useState(false);

  if (!article) return null;

  const pubmedURL = `https://pubmed.ncbi.nlm.nih.gov/${article.pmid}/`;

  const copyPubMedLink = async () => {
    try {
      await navigator.clipboard.writeText(pubmedURL);
      setCopied(true);
      toast.success("PubMed link copied");
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Copy failed — select the link manually");
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="flex flex-col gap-0 p-0">
        <SheetHeader className="border-b p-6 pr-14">
          <SheetTitle className="text-base leading-snug">{article.title}</SheetTitle>
          <SheetDescription className="text-xs">
            PMID <span className="font-mono">{article.pmid}</span>
            {article.journal && <> · {article.journal}</>}
            {article.publication_date && <> · {formatDate(article.publication_date)}</>}
          </SheetDescription>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto p-6 space-y-5">
          <div className="flex flex-wrap gap-1.5">
            {article.matched_queries.map((q) => (
              <Badge key={q.id} variant="secondary">
                {q.name}
              </Badge>
            ))}
          </div>

          <section>
            <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Authors
            </h4>
            <p className="text-sm">{authorsSummary(article.authors, 50) || "—"}</p>
          </section>

          {article.publication_types.length > 0 && (
            <section>
              <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Publication types
              </h4>
              <div className="flex flex-wrap gap-1.5">
                {article.publication_types.map((t) => (
                  <Badge key={t} variant="outline">
                    {t}
                  </Badge>
                ))}
              </div>
            </section>
          )}

          <Separator />

          <section>
            <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Abstract
            </h4>
            <div className="whitespace-pre-wrap text-sm leading-relaxed">
              {article.abstract ?? (
                <span className="italic text-muted-foreground">
                  No abstract available.
                </span>
              )}
            </div>
          </section>

          <Separator />

          <section className="space-y-1 text-xs text-muted-foreground">
            <p>Fetched {relativeTime(article.fetched_at)}</p>
          </section>
        </div>

        <div className="flex gap-2 border-t bg-card p-4">
          <Button asChild variant="outline" className="flex-1">
            <a href={pubmedURL} target="_blank" rel="noreferrer">
              <ExternalLink />
              Open on PubMed
            </a>
          </Button>
          <Button variant="outline" onClick={copyPubMedLink}>
            <LinkIcon />
            {copied ? "Copied" : "Copy link"}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
