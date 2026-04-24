import { useState } from "react";
import { Check, ChevronDown, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

/**
 * Curated list of commonly-relevant publication types for clinical literature
 * alerts. Matches PubMed's MeSH publication-type vocabulary. Exact-case, since
 * the backend filter is a case-sensitive string comparison.
 */
const CURATED_TYPES: readonly string[] = [
  "Journal Article",
  "Review",
  "Meta-Analysis",
  "Systematic Review",
  "Randomized Controlled Trial",
  "Clinical Trial",
  "Multicenter Study",
  "Observational Study",
  "Case Reports",
  "Practice Guideline",
  "Comment",
  "Letter",
  "Editorial",
  "Published Erratum",
  "Retraction of Publication",
  "Retracted Publication",
  "News",
  "Biography",
];

interface Props {
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  id?: string;
}

export function PublicationTypesCombobox({ value, onChange, placeholder = "Pick publication types…", id }: Props) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const toggle = (type: string) => {
    if (value.includes(type)) onChange(value.filter((v) => v !== type));
    else onChange([...value, type]);
  };

  const remove = (type: string) => onChange(value.filter((v) => v !== type));

  const searchTrimmed = search.trim();
  const filteredCurated = CURATED_TYPES.filter((t) =>
    t.toLowerCase().includes(searchTrimmed.toLowerCase()),
  );
  const curatedHasExact = CURATED_TYPES.some(
    (t) => t.toLowerCase() === searchTrimmed.toLowerCase(),
  );
  const alreadySelected = value.some(
    (v) => v.toLowerCase() === searchTrimmed.toLowerCase(),
  );
  const showCustomOption = searchTrimmed.length > 0 && !curatedHasExact && !alreadySelected;

  return (
    <div className="space-y-2">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id={id}
            type="button"
            variant="outline"
            role="combobox"
            className="w-full justify-between font-normal"
          >
            <span className="truncate text-muted-foreground">{placeholder}</span>
            <ChevronDown className="size-4 opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
          <Command shouldFilter={false}>
            <CommandInput placeholder="Type to search…" value={search} onValueChange={setSearch} />
            <CommandList>
              {filteredCurated.length === 0 && !showCustomOption && (
                <CommandEmpty>No publication types found.</CommandEmpty>
              )}
              {filteredCurated.length > 0 && (
                <CommandGroup>
                  {filteredCurated.map((type) => {
                    const selected = value.includes(type);
                    return (
                      <CommandItem
                        key={type}
                        value={type}
                        onSelect={() => toggle(type)}
                      >
                        <Check
                          className={cn(
                            "size-4",
                            selected ? "opacity-100" : "opacity-0",
                          )}
                        />
                        {type}
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              )}
              {showCustomOption && (
                <CommandGroup heading="Custom">
                  <CommandItem
                    value={`__custom__${searchTrimmed}`}
                    onSelect={() => {
                      toggle(searchTrimmed);
                      setSearch("");
                    }}
                  >
                    Use <span className="font-medium">"{searchTrimmed}"</span>
                  </CommandItem>
                </CommandGroup>
              )}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>

      {value.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {value.map((v) => (
            <Badge key={v} variant="secondary" className="gap-1 pr-1">
              {v}
              <button
                type="button"
                onClick={() => remove(v)}
                className="rounded-sm hover:bg-muted/60"
                aria-label={`Remove ${v}`}
              >
                <X className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}
