import { createFileRoute } from "@tanstack/react-router";
import StatusView from "@/views/StatusView";

export const Route = createFileRoute("/")({
  component: StatusView,
});