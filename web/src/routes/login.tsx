import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ThemeToggle } from "@/components/ThemeToggle";
import { login, type User } from "@/api/auth";
import { APIError } from "@/api/client";
import { authQueryOptions } from "@/hooks/useAuth";

const loginSchema = z.object({
  username: z.string().min(1, "Username is required"),
  password: z.string().min(1, "Password is required"),
});
type LoginForm = z.infer<typeof loginSchema>;

const loginSearchSchema = z.object({
  redirect: z.string().optional(),
});

export const Route = createFileRoute("/login")({
  validateSearch: loginSearchSchema,
  beforeLoad: async ({ context, search }) => {
    // If already authenticated, skip login and go to the landing page.
    const user = await context.queryClient.ensureQueryData(authQueryOptions);
    if (user) {
      throw redirect({ to: search.redirect ?? "/queries" });
    }
  },
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const search = Route.useSearch();
  const queryClient = useQueryClient();

  const form = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: "", password: "" },
  });

  const mutation = useMutation({
    mutationFn: login,
    onSuccess: async (user: User) => {
      queryClient.setQueryData(authQueryOptions.queryKey, user);
      await navigate({ to: search.redirect ?? "/queries" });
    },
    onError: (error: unknown) => {
      if (error instanceof APIError) {
        if (error.fields?.length) {
          for (const f of error.fields) {
            if (f.field === "username" || f.field === "password") {
              form.setError(f.field, { message: f.message });
            }
          }
          return;
        }
        if (error.status === 401) {
          toast.error("Invalid username or password");
          return;
        }
        if (error.status === 429) {
          toast.error("Too many attempts — try again in a few minutes");
          return;
        }
        toast.error(error.message);
        return;
      }
      toast.error("Network error — please try again");
    },
  });

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-background text-foreground">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>

      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Sign in</CardTitle>
          <CardDescription>PubMed Alerts</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={form.handleSubmit((data) => mutation.mutate(data))}
            className="space-y-4"
            noValidate
          >
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                autoComplete="username"
                autoFocus
                {...form.register("username")}
                aria-invalid={!!form.formState.errors.username}
              />
              {form.formState.errors.username && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.username.message}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                {...form.register("password")}
                aria-invalid={!!form.formState.errors.password}
              />
              {form.formState.errors.password && (
                <p className="text-sm text-destructive">
                  {form.formState.errors.password.message}
                </p>
              )}
            </div>

            <Button type="submit" className="w-full" disabled={mutation.isPending}>
              {mutation.isPending ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
