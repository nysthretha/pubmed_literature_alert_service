import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Key, Plus, Shield, ShieldOff, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ErrorState } from "@/components/shared/ErrorState";
import { PageHeader } from "@/components/shared/PageHeader";
import {
  deleteUser,
  listUsers,
  updateUser,
  type AdminUser,
} from "@/api/admin";
import { APIError } from "@/api/client";
import { useAuth } from "@/hooks/useAuth";
import { formatDate, relativeTime } from "@/lib/format";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { UserFormDialog } from "./UserFormDialog";
import { ResetPasswordDialog } from "./ResetPasswordDialog";

export function UsersTab() {
  const { data: me } = useAuth();
  const queryClient = useQueryClient();

  const [createOpen, setCreateOpen] = useState(false);
  const [resetFor, setResetFor] = useState<AdminUser | null>(null);
  const [deleteFor, setDeleteFor] = useState<AdminUser | null>(null);

  const usersQuery = useQuery({
    queryKey: ["admin", "users"],
    queryFn: () => listUsers({ limit: 100 }),
  });

  const toggleAdmin = useMutation({
    mutationFn: (u: AdminUser) => updateUser(u.id, { is_admin: !u.is_admin }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
      toast.success("User updated");
    },
    onError: (err) => toast.error(err instanceof APIError ? err.message : "Update failed"),
  });

  const deleteMutation = useMutation({
    mutationFn: async (u: AdminUser) => {
      await deleteUser(u.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
      toast.success("User deleted");
      setDeleteFor(null);
    },
    onError: (err) => toast.error(err instanceof APIError ? err.message : "Delete failed"),
  });

  return (
    <Card>
      <CardContent className="space-y-4 pt-6">
        <PageHeader
          title="Users"
          description="All accounts on this server."
          actions={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus />
              New user
            </Button>
          }
        />

        {usersQuery.isLoading && (
          <div className="space-y-2">
            <Skeleton className="h-12" />
            <Skeleton className="h-12" />
          </div>
        )}

        {usersQuery.error && (
          <ErrorState
            message={
              usersQuery.error instanceof APIError
                ? usersQuery.error.message
                : String(usersQuery.error)
            }
            onRetry={() => usersQuery.refetch()}
          />
        )}

        {usersQuery.data && (
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Queries</TableHead>
                  <TableHead>Last login</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-[120px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersQuery.data.users.map((u) => {
                  const isSelf = me?.id === u.id;
                  return (
                    <TableRow key={u.id}>
                      <TableCell>
                        <div className="space-y-0.5">
                          <div className="flex items-center gap-2 font-medium">
                            {u.username}
                            {u.is_admin && <Badge variant="outline">Admin</Badge>}
                            {isSelf && <Badge variant="secondary">You</Badge>}
                          </div>
                          <div className="text-xs text-muted-foreground">{u.email}</div>
                        </div>
                      </TableCell>
                      <TableCell className="text-sm">{u.queries_count}</TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {relativeTime(u.last_login_at)}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {formatDate(u.created_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => setResetFor(u)}
                                aria-label="Reset password"
                              >
                                <Key />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Reset password</TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => toggleAdmin.mutate(u)}
                                disabled={toggleAdmin.isPending || isSelf}
                                aria-label={u.is_admin ? "Revoke admin" : "Grant admin"}
                              >
                                {u.is_admin ? <ShieldOff /> : <Shield />}
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>
                              {u.is_admin ? "Revoke admin" : "Grant admin"}
                            </TooltipContent>
                          </Tooltip>
                          {!isSelf && (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  onClick={() => setDeleteFor(u)}
                                  className="text-muted-foreground hover:text-destructive"
                                  aria-label="Delete user"
                                >
                                  <Trash2 />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>Delete user</TooltipContent>
                            </Tooltip>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <UserFormDialog open={createOpen} onOpenChange={setCreateOpen} />
      <ResetPasswordDialog
        open={resetFor !== null}
        onOpenChange={(o) => !o && setResetFor(null)}
        user={resetFor}
      />
      <AlertDialog
        open={deleteFor !== null}
        onOpenChange={(o) => !o && setDeleteFor(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete user?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium">{deleteFor?.username}</span> and
              their queries, digest history, and sessions will be removed.
              Articles are shared across users and remain. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                if (deleteFor) deleteMutation.mutate(deleteFor);
              }}
              className={cn(buttonVariants({ variant: "destructive" }))}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? "Deleting…" : "Delete user"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
