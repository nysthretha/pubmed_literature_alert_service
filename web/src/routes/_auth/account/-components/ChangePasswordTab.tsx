import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword } from "@/api/auth";
import { APIError } from "@/api/client";

const schema = z
  .object({
    current_password: z.string().min(1, "Current password is required"),
    new_password: z.string().min(8, "Must be at least 8 characters"),
    confirm_new_password: z.string().min(1, "Please re-enter the new password"),
  })
  .refine((d) => d.new_password === d.confirm_new_password, {
    path: ["confirm_new_password"],
    message: "Passwords do not match",
  });
type FormValues = z.infer<typeof schema>;

export function ChangePasswordTab() {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { current_password: "", new_password: "", confirm_new_password: "" },
  });

  const mutation = useMutation({
    mutationFn: changePassword,
    onSuccess: () => {
      toast.success("Password changed");
      form.reset();
    },
    onError: (err) => {
      if (err instanceof APIError) {
        if (err.status === 401) {
          form.setError("current_password", { message: "Current password is incorrect" });
          return;
        }
        if (err.status === 429) {
          toast.error("Too many attempts — try again in a few minutes");
          return;
        }
        if (err.fields?.length) {
          for (const f of err.fields) {
            form.setError(f.field as keyof FormValues, { message: f.message });
          }
          return;
        }
        toast.error(err.message);
        return;
      }
      toast.error("Network error — try again");
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Change password</CardTitle>
        <CardDescription>
          You'll stay signed in after changing your password.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit((v) =>
            mutation.mutate({
              current_password: v.current_password,
              new_password: v.new_password,
            }),
          )}
          className="max-w-md space-y-4"
          noValidate
        >
          <div className="space-y-2">
            <Label htmlFor="current-password">Current password</Label>
            <Input
              id="current-password"
              type="password"
              autoComplete="current-password"
              {...form.register("current_password")}
            />
            {form.formState.errors.current_password && (
              <p className="text-sm text-destructive">
                {form.formState.errors.current_password.message}
              </p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">New password</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              {...form.register("new_password")}
            />
            {form.formState.errors.new_password && (
              <p className="text-sm text-destructive">
                {form.formState.errors.new_password.message}
              </p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-new-password">Confirm new password</Label>
            <Input
              id="confirm-new-password"
              type="password"
              autoComplete="new-password"
              {...form.register("confirm_new_password")}
            />
            {form.formState.errors.confirm_new_password && (
              <p className="text-sm text-destructive">
                {form.formState.errors.confirm_new_password.message}
              </p>
            )}
          </div>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? "Updating…" : "Update password"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
