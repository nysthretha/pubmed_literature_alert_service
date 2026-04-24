import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { triggerDigest } from "@/api/digests";
import { APIError } from "@/api/client";

export function SendTestDigestButton() {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: triggerDigest,
    onSuccess: async () => {
      // Give the worker a moment to process the trigger, then refresh.
      window.setTimeout(
        () => queryClient.invalidateQueries({ queryKey: ["digests"] }),
        1500,
      );
    },
  });

  const onClick = () => {
    toast.promise(mutation.mutateAsync(), {
      loading: "Queuing digest…",
      success: "Digest trigger queued. Refresh the list in a few seconds.",
      error: (err: unknown) =>
        err instanceof APIError ? err.message : "Trigger failed",
    });
  };

  return (
    <Button variant="outline" onClick={onClick} disabled={mutation.isPending}>
      <Send />
      Send digest now
    </Button>
  );
}
