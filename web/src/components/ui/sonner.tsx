import { Toaster as SonnerToaster } from "sonner";
import { useTheme } from "@/hooks/useTheme";

/**
 * Theme-aware Sonner Toaster. Mount once in the root route.
 * Position: bottom-right (sonner's default, least intrusive for a sidebar-
 * layout app). Pair with the `richColors` option so destructive toasts
 * don't require custom className work.
 */
export function Toaster() {
  const { theme } = useTheme();
  return (
    <SonnerToaster
      theme={theme}
      position="bottom-right"
      richColors
      closeButton
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-background group-[.toaster]:text-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg",
          description: "group-[.toast]:text-muted-foreground",
        },
      }}
    />
  );
}
