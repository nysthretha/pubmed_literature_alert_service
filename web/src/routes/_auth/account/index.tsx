import { createFileRoute } from "@tanstack/react-router";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/shared/PageHeader";
import { useAuth } from "@/hooks/useAuth";
import { ProfileTab } from "./-components/ProfileTab";
import { ChangePasswordTab } from "./-components/ChangePasswordTab";
import { UsersTab } from "./-components/UsersTab";

export const Route = createFileRoute("/_auth/account/")({
  component: AccountPage,
});

function AccountPage() {
  const { data: user } = useAuth();
  const isAdmin = user?.is_admin === true;

  return (
    <div className="space-y-6">
      <PageHeader title="Account" description="Your profile and security settings." />

      <Tabs defaultValue="profile" className="space-y-4">
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="password">Change password</TabsTrigger>
          {isAdmin && <TabsTrigger value="users">Users</TabsTrigger>}
        </TabsList>
        <TabsContent value="profile">
          <ProfileTab />
        </TabsContent>
        <TabsContent value="password">
          <ChangePasswordTab />
        </TabsContent>
        {isAdmin && (
          <TabsContent value="users">
            <UsersTab />
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
