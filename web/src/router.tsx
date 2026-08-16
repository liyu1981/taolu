import { createRootRoute, createRoute, createRouter, Outlet } from "@tanstack/react-router";
import { Link } from "@tanstack/react-router";
import { Archive, BookOpen, Database } from "lucide-react";
import { cn } from "@/lib/utils";
import StatusView from "@/views/StatusView";
import BrowseView from "@/views/BrowseView";
import TaoluDetailView from "@/views/TaoluDetailView";

function Layout() {
  const links = [
    { to: "/", label: "Status", icon: Database },
    { to: "/taolu", label: "Browse", icon: BookOpen },
  ];
  return (
    <div className="min-h-screen bg-background">
      <header className="border-b sticky top-0 z-10 bg-background/95 backdrop-blur">
        <div className="mx-auto max-w-6xl flex items-center gap-6 px-4 py-3">
          <div className="flex items-center gap-2 font-semibold">
            <Archive className="h-5 w-5" />
            <span>taolu</span>
          </div>
          <nav className="flex items-center gap-1">
            {links.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                className={cn(
                  "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground",
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
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}

const rootRoute = createRootRoute({
  component: Layout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: StatusView,
});

const browseRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/taolu",
  component: BrowseView,
});

const detailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/taolu/$name",
  component: TaoluDetailView,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  browseRoute,
  detailRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
