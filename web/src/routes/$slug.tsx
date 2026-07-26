import { createFileRoute } from "@tanstack/react-router"

export const Route = createFileRoute("/$slug")({
  component: SlugPage,
})

function SlugPage() {
  const { slug } = Route.useParams()

  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-semibold">Clipboard placeholder</h1>
      <p className="text-muted-foreground">
        Route parameter: <code className="rounded bg-muted px-2 py-1">{slug}</code>
      </p>
    </section>
  )
}
