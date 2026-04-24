import { createFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/hooks/useAuth";

export const Route = createFileRoute("/_auth/account")({
  component: AccountPage,
});

function AccountPage() {
  const { data: user } = useAuth();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Account</h1>
        <p className="text-sm text-muted-foreground">Signed-in user details.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Profile</CardTitle>
          <CardDescription>
            Password changes and account edits arrive in a later milestone —
            for now use the CLI <code>./scheduler reset-password</code>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Field label="Username" value={user?.username} />
          <Field label="Email" value={user?.email} />
          <Field label="Role" value={user?.is_admin ? "Administrator" : "User"} />
        </CardContent>
      </Card>
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value ?? "—"}</span>
    </div>
  );
}
