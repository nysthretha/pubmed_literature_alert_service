import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { BookOpen, FileText, LogOut, Mail, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/ThemeToggle";
import { logout } from "@/api/auth";
import { authQueryOptions, useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/queries", label: "Queries", icon: BookOpen },
  { to: "/articles", label: "Articles", icon: FileText },
  { to: "/digests", label: "Digests", icon: Mail },
  { to: "/account", label: "Account", icon: Settings },
] as const;

export function AppShell() {
  const { data: user } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      queryClient.setQueryData(authQueryOptions.queryKey, null);
      // Evict anything else that might be scoped to the previous session.
      queryClient.clear();
      await navigate({ to: "/login" });
    },
    onError: () => {
      toast.error("Logout failed. Please try again.");
    },
  });

  return (
    <div className="min-h-screen flex bg-background text-foreground">
      <aside className="w-60 border-r bg-card flex flex-col">
        <div className="h-14 flex items-center px-4 border-b">
          <span className="font-semibold tracking-tight">PubMed Alerts</span>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {navItems.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="flex items-center gap-2 px-3 py-2 text-sm rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
              activeProps={{
                className: cn("bg-accent text-accent-foreground"),
              }}
            >
              <Icon className="size-4" />
              {label}
            </Link>
          ))}
        </nav>
        <div className="p-2 border-t text-xs text-muted-foreground">
          {user && (
            <div className="px-3 py-2 flex flex-col gap-0.5">
              <span className="font-medium text-foreground">{user.username}</span>
              <span>{user.email}</span>
              {user.is_admin && (
                <span className="uppercase tracking-wider text-[10px] mt-1">
                  Admin
                </span>
              )}
            </div>
          )}
        </div>
      </aside>

      <div className="flex-1 flex flex-col">
        <header className="h-14 flex items-center justify-end gap-2 px-4 border-b bg-card">
          <ThemeToggle />
          <Button
            variant="ghost"
            size="sm"
            onClick={() => logoutMutation.mutate()}
            disabled={logoutMutation.isPending}
          >
            <LogOut className="size-4" />
            Sign out
          </Button>
        </header>
        <main className="flex-1 p-6 overflow-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
