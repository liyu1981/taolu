import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import { Archive, BookOpen, Database } from "lucide-react";
import { cn } from "@/lib/utils";
import { ParticleBackdrop } from "@/components/particle-backdrop";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  const links = [
    { to: "/", label: "Status", icon: Database },
    { to: "/taolu", label: "Browse", icon: BookOpen },
  ];
  return (
    <div className="min-h-screen ambient-bg">
      <ParticleBackdrop />
      <header className="apple-panel sticky top-0 z-10">
        <div className="mx-auto max-w-6xl flex items-center gap-6 px-4 py-3">
          <div className="flex items-center gap-2 font-semibold tracking-tight">
            <span className="flex h-7 w-7 items-center justify-center rounded-lg glass-control text-foreground">
              <Archive className="h-4 w-4" />
            </span>
            <span>taolu</span>
          </div>
          <nav className="flex items-center gap-1">
            {links.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                className={cn(
                  "flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-muted-foreground transition-[background-color,color] duration-150 hover:bg-accent hover:text-foreground",
                )}
                activeProps={{ className: "bg-accent text-foreground" }}
              >
                <Icon className="h-4 w-4" />
                {label}
              </Link>
            ))}
          </nav>
        </div>
      </header>
      <main className="relative z-10 mx-auto max-w-6xl px-4 py-6">
        <div className="rounded-3xl apple-content bg-clip-padding p-6 sm:p-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}