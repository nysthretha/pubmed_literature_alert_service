import { Link, Outlet } from "@tanstack/react-router";
import { BookOpen, FileText, Mail, Settings } from "lucide-react";
import { ThemeToggle } from "@/components/ThemeToggle";
import { UserMenu } from "@/components/UserMenu";
import { TooltipProvider } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/queries", label: "Queries", icon: BookOpen },
  { to: "/articles", label: "Articles", icon: FileText },
  { to: "/digests", label: "Digests", icon: Mail },
  { to: "/account", label: "Account", icon: Settings },
] as const;

export function AppShell() {
  return (
    <TooltipProvider delayDuration={200}>
      <div className="min-h-screen flex bg-background text-foreground">
        <aside className="hidden md:flex w-60 border-r bg-card flex-col">
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
        </aside>

        <div className="flex-1 flex flex-col min-w-0">
          <header className="h-14 flex items-center justify-end gap-1 px-4 border-b bg-card">
            <ThemeToggle />
            <UserMenu />
          </header>
          <main className="flex-1 p-6 overflow-auto">
            <Outlet />
          </main>
        </div>
      </div>
    </TooltipProvider>
  );
}
