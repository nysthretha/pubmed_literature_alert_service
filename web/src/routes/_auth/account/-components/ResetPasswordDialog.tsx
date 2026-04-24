import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
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
import { resetUserPassword, type AdminUser } from "@/api/admin";
import { APIError } from "@/api/client";

const schema = z.object({
  new_password: z.string().min(12, "At least 12 characters"),
});
type FormValues = z.infer<typeof schema>;

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: AdminUser | null;
}

export function ResetPasswordDialog({ open, onOpenChange, user }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { new_password: "" },
  });

  useEffect(() => {
    if (!open) form.reset();
  }, [open, form]);

  const mutation = useMutation({
    mutationFn: async (v: FormValues) => {
      if (!user) throw new Error("no user");
      await resetUserPassword(user.id, v.new_password);
    },
    onSuccess: () => {
      toast.success(user ? `Password reset for ${user.username}` : "Password reset");
      onOpenChange(false);
      form.reset();
    },
    onError: (err) => {
      if (err instanceof APIError && err.fields?.length) {
        for (const f of err.fields) {
          form.setError(f.field as keyof FormValues, { message: f.message });
        }
        return;
      }
      toast.error(err instanceof APIError ? err.message : "Reset failed");
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>
            Set a new password for <span className="font-medium">{user?.username}</span>.
            They'll need it on next sign-in.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={form.handleSubmit((v) => mutation.mutate(v))} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="rp-password">New password (min 12)</Label>
            <Input
              id="rp-password"
              type="password"
              autoFocus
              autoComplete="new-password"
              {...form.register("new_password")}
            />
            {form.formState.errors.new_password && (
              <p className="text-sm text-destructive">
                {form.formState.errors.new_password.message}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Saving…" : "Reset password"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
