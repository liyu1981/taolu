import { createFileRoute } from "@tanstack/react-router";
import TaoluDetailView from "@/views/TaoluDetailView";

export const Route = createFileRoute("/browse/$name")({
  component: TaoluDetailView,
});
