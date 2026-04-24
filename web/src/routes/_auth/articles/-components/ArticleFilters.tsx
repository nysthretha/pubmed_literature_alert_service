import { useState, useEffect } from "react";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select-inline";
import type { Query } from "@/api/queries";

export interface Filters {
  search: string;
  queryId: number | null;
  sinceDays: number | null; // null = all
}

interface Props {
  filters: Filters;
  onChange: (next: Filters) => void;
  queries: Query[];
}

export function ArticleFilters({ filters, onChange, queries }: Props) {
  const [searchInput, setSearchInput] = useState(filters.search);

  // Debounce the search input so we're not issuing a request per keystroke.
  useEffect(() => {
    if (searchInput === filters.search) return;
    const h = window.setTimeout(() => onChange({ ...filters, search: searchInput }), 300);
    return () => window.clearTimeout(h);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput]);

  const hasActive =
    filters.search.length > 0 || filters.queryId !== null || filters.sinceDays !== null;

  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="flex-1 min-w-[240px] space-y-1">
        <Label htmlFor="a-search" className="text-xs text-muted-foreground">
          Search title / abstract
        </Label>
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
          <Input
            id="a-search"
            className="pl-8"
            placeholder="e.g. troponin"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
          />
        </div>
      </div>

      <div className="w-52 space-y-1">
        <Label className="text-xs text-muted-foreground">Query</Label>
        <Select
          value={filters.queryId === null ? "all" : String(filters.queryId)}
          onValueChange={(v) =>
            onChange({ ...filters, queryId: v === "all" ? null : Number(v) })
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All queries</SelectItem>
            {queries.map((q) => (
              <SelectItem key={q.id} value={String(q.id)}>
                {q.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="w-40 space-y-1">
        <Label className="text-xs text-muted-foreground">Fetched since</Label>
        <Select
          value={filters.sinceDays === null ? "all" : String(filters.sinceDays)}
          onValueChange={(v) =>
            onChange({ ...filters, sinceDays: v === "all" ? null : Number(v) })
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any time</SelectItem>
            <SelectItem value="1">Last 24 hours</SelectItem>
            <SelectItem value="7">Last 7 days</SelectItem>
            <SelectItem value="30">Last 30 days</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {hasActive && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            setSearchInput("");
            onChange({ search: "", queryId: null, sinceDays: null });
          }}
        >
          <X />
          Clear filters
        </Button>
      )}
    </div>
  );
}

// Compute RFC3339 `since` from days-ago.
export function sinceParam(days: number | null): string | undefined {
  if (days === null) return undefined;
  return new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString();
}
