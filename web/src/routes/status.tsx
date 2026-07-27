import { createFileRoute } from "@tanstack/react-router"
import { StatusView } from "@/components/status-view"

export const Route = createFileRoute("/status")({
  component: StatusPage,
})

function StatusPage() {
  return <StatusView />
}
