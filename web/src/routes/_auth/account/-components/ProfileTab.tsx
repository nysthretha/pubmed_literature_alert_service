import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/hooks/useAuth";

export function ProfileTab() {
  const { data: user } = useAuth();
  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>
          Read-only for now. Email edits arrive in a later milestone; admins
          can update users' emails via the Users tab.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <Field label="Username" value={user?.username} />
        <Field label="Email" value={user?.email} />
        <Field label="Role" value={user?.is_admin ? "Administrator" : "User"} />
      </CardContent>
    </Card>
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
