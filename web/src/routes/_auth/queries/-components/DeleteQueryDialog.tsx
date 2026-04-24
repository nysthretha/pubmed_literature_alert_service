import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { deleteQuery, type Query } from "@/api/queries";
import { APIError } from "@/api/client";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  query: Query | null;
}

export function DeleteQueryDialog({ open, onOpenChange, query }: Props) {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async () => {
      if (!query) throw new Error("no query");
      await deleteQuery(query.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["queries"] });
      toast.success("Query deleted");
      onOpenChange(false);
    },
    onError: (err) => {
      toast.error(err instanceof APIError ? err.message : "Delete failed");
    },
  });

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete query?</AlertDialogTitle>
          <AlertDialogDescription className="space-y-2">
            <span className="block">
              <span className="font-medium">{query?.name}</span> and its{" "}
              {query?.article_count ?? 0} matched-article references will be
              removed.
            </span>
            <span className="block">
              The articles themselves stay — they may match your other queries,
              or other users' queries.
            </span>
            <span className="block">This cannot be undone.</span>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={mutation.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.preventDefault();
              mutation.mutate();
            }}
            disabled={mutation.isPending}
            className={cn(buttonVariants({ variant: "destructive" }))}
          >
            {mutation.isPending ? "Deleting…" : "Delete query"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
