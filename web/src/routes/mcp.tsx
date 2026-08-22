import { createFileRoute } from "@tanstack/react-router";
import McpToolsView from "@/views/McpToolsView";

export const Route = createFileRoute("/mcp")({
  component: McpToolsView,
});
