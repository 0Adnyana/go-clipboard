import { createFileRoute } from "@tanstack/react-router"
import { StatusView } from "@/components/status-view"

export const Route = createFileRoute("/")({
  component: IndexPage,
})

function IndexPage() {
  return <StatusView />
}
