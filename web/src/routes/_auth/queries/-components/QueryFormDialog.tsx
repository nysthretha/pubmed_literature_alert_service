import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { PublicationTypesCombobox } from "@/components/PublicationTypesCombobox";
import { APIError } from "@/api/client";
import {
  createQuery,
  updateQuery,
  type CreateQueryInput,
  type Query,
  type UpdateQueryInput,
} from "@/api/queries";

const MIN_POLL_INTERVAL_SECONDS = 3600;
const DEFAULT_POLL_INTERVAL_SECONDS = 21_600;

const schema = z.object({
  name: z.string().min(1, "Name is required").max(100, "Must be 100 characters or fewer"),
  query_string: z
    .string()
    .min(1, "Query string is required")
    .max(500, "Must be 500 characters or fewer"),
  poll_interval_seconds: z
    .number({ invalid_type_error: "Must be a number" })
    .int("Must be a whole number")
    .min(MIN_POLL_INTERVAL_SECONDS, `Must be >= ${MIN_POLL_INTERVAL_SECONDS} (1 hour)`),
  min_abstract_length: z
    .number({ invalid_type_error: "Must be a number" })
    .int("Must be a whole number")
    .min(0, "Must be >= 0"),
  is_active: z.boolean(),
  publication_type_allowlist: z.array(z.string()),
  publication_type_blocklist: z.array(z.string()),
  notes: z.string(),
});
type FormValues = z.infer<typeof schema>;

const DEFAULT_BLOCKLIST = ["Comment", "Retraction of Publication", "Published Erratum"];

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: "create" | "edit";
  query?: Query;
}

export function QueryFormDialog({ open, onOpenChange, mode, query }: Props) {
  const queryClient = useQueryClient();

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      query_string: "",
      poll_interval_seconds: DEFAULT_POLL_INTERVAL_SECONDS,
      min_abstract_length: 0,
      is_active: true,
      publication_type_allowlist: [],
      publication_type_blocklist: DEFAULT_BLOCKLIST,
      notes: "",
    },
  });

  useEffect(() => {
    if (!open) return;
    if (mode === "edit" && query) {
      form.reset({
        name: query.name,
        query_string: query.query_string,
        poll_interval_seconds: query.poll_interval_seconds,
        min_abstract_length: query.min_abstract_length,
        is_active: query.is_active,
        publication_type_allowlist: query.publication_type_allowlist ?? [],
        publication_type_blocklist: query.publication_type_blocklist ?? [],
        notes: query.notes ?? "",
      });
    } else {
      form.reset();
    }
  }, [open, mode, query, form]);

  const mutation = useMutation({
    mutationFn: async (values: FormValues) => {
      const payload: CreateQueryInput = {
        name: values.name.trim(),
        query_string: values.query_string.trim(),
        poll_interval_seconds: values.poll_interval_seconds,
        is_active: values.is_active,
        min_abstract_length: values.min_abstract_length,
        publication_type_allowlist:
          values.publication_type_allowlist.length > 0
            ? values.publication_type_allowlist
            : null,
        publication_type_blocklist: values.publication_type_blocklist,
        notes: values.notes.trim() || null,
      };
      if (mode === "edit" && query) {
        const patch: UpdateQueryInput = payload;
        return updateQuery(query.id, patch);
      }
      return createQuery(payload);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["queries"] });
      toast.success(mode === "edit" ? "Query updated" : "Query created");
      onOpenChange(false);
    },
    onError: (err) => {
      if (err instanceof APIError) {
        if (err.fields?.length) {
          for (const f of err.fields) {
            form.setError(f.field as keyof FormValues, { message: f.message });
          }
          return;
        }
        if (err.status === 409) {
          form.setError("name", { message: err.message });
          return;
        }
        toast.error(err.message);
        return;
      }
      toast.error("Network error — try again");
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{mode === "edit" ? "Edit query" : "Create query"}</DialogTitle>
          <DialogDescription>
            PubMed search terms use the standard E-utilities syntax (e.g. tags
            like <code>[tiab]</code>, <code>[mh]</code>). Publication types are
            case-sensitive.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
          className="grid gap-4 max-h-[70vh] overflow-y-auto pr-1"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="q-name">Name</Label>
            <Input id="q-name" autoFocus {...form.register("name")} />
            {form.formState.errors.name && (
              <p className="text-sm text-destructive">{form.formState.errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="q-string">Query string</Label>
            <Textarea
              id="q-string"
              rows={3}
              className="font-mono text-xs"
              {...form.register("query_string")}
            />
            {form.formState.errors.query_string && (
              <p className="text-sm text-destructive">
                {form.formState.errors.query_string.message}
              </p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="q-interval">
                Poll interval (seconds)
                <span className="ml-1 text-xs text-muted-foreground">21600 = 6h</span>
              </Label>
              <Input
                id="q-interval"
                type="number"
                min={MIN_POLL_INTERVAL_SECONDS}
                {...form.register("poll_interval_seconds", { valueAsNumber: true })}
              />
              {form.formState.errors.poll_interval_seconds && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.poll_interval_seconds.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="q-abslen">Min abstract length</Label>
              <Input
                id="q-abslen"
                type="number"
                min={0}
                {...form.register("min_abstract_length", { valueAsNumber: true })}
              />
              {form.formState.errors.min_abstract_length && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.min_abstract_length.message}
                </p>
              )}
            </div>
          </div>

          <div className="space-y-2">
            <Label>Publication types — allowlist (optional)</Label>
            <PublicationTypesCombobox
              value={form.watch("publication_type_allowlist")}
              onChange={(v) => form.setValue("publication_type_allowlist", v, { shouldDirty: true })}
              placeholder="Leave empty to allow all publication types"
            />
          </div>

          <div className="space-y-2">
            <Label>Publication types — blocklist</Label>
            <PublicationTypesCombobox
              value={form.watch("publication_type_blocklist")}
              onChange={(v) => form.setValue("publication_type_blocklist", v, { shouldDirty: true })}
              placeholder="Block these publication types"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="q-notes">Notes</Label>
            <Textarea id="q-notes" rows={2} {...form.register("notes")} />
          </div>

          {mode === "edit" && (
            <div className="flex items-center justify-between rounded-md border p-3">
              <div>
                <Label htmlFor="q-active">Active</Label>
                <p className="text-xs text-muted-foreground">
                  Inactive queries are skipped on each scheduler tick.
                </p>
              </div>
              <Switch
                id="q-active"
                checked={form.watch("is_active")}
                onCheckedChange={(v) => form.setValue("is_active", v, { shouldDirty: true })}
              />
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Saving…" : mode === "edit" ? "Save" : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
