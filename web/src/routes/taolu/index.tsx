import { createFileRoute } from "@tanstack/react-router";
import BrowseView from "@/views/BrowseView";

export const Route = createFileRoute("/taolu/")({
  component: BrowseView,
});