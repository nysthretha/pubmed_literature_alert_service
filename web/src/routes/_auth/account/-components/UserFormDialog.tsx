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
import { Switch } from "@/components/ui/switch";
import { createUser } from "@/api/admin";
import { APIError } from "@/api/client";

const schema = z.object({
  username: z
    .string()
    .min(3, "At least 3 characters")
    .max(40, "At most 40 characters"),
  email: z.string().email("Must be a valid email address"),
  password: z.string().min(12, "At least 12 characters"),
  is_admin: z.boolean(),
});
type FormValues = z.infer<typeof schema>;

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UserFormDialog({ open, onOpenChange }: Props) {
  const queryClient = useQueryClient();
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { username: "", email: "", password: "", is_admin: false },
  });

  const mutation = useMutation({
    mutationFn: createUser,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
      toast.success("User created");
      onOpenChange(false);
      form.reset();
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
          form.setError("username", { message: err.message });
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
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create user</DialogTitle>
          <DialogDescription>
            New users cannot register themselves — admins must create accounts.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
          className="space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="u-username">Username</Label>
            <Input id="u-username" autoFocus {...form.register("username")} />
            {form.formState.errors.username && (
              <p className="text-sm text-destructive">
                {form.formState.errors.username.message}
              </p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="u-email">Email</Label>
            <Input id="u-email" type="email" {...form.register("email")} />
            {form.formState.errors.email && (
              <p className="text-sm text-destructive">
                {form.formState.errors.email.message}
              </p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="u-password">Password (min 12)</Label>
            <Input
              id="u-password"
              type="password"
              autoComplete="new-password"
              {...form.register("password")}
            />
            {form.formState.errors.password && (
              <p className="text-sm text-destructive">
                {form.formState.errors.password.message}
              </p>
            )}
          </div>
          <div className="flex items-center justify-between rounded-md border p-3">
            <div>
              <Label htmlFor="u-admin">Administrator</Label>
              <p className="text-xs text-muted-foreground">
                Grants access to the Users tab and admin endpoints.
              </p>
            </div>
            <Switch
              id="u-admin"
              checked={form.watch("is_admin")}
              onCheckedChange={(v) => form.setValue("is_admin", v, { shouldDirty: true })}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? "Creating…" : "Create user"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
