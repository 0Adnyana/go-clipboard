import { createFileRoute } from "@tanstack/react-router"
import { useEffect, useState } from "react"
import { ApiError, readClip, type ReadClipResponse } from "@/lib/api"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export const Route = createFileRoute("/$slug")({
  head: () => ({
    meta: [{ name: "robots", content: "noindex" }],
  }),
  component: SlugPage,
})

type LoadState =
  | { kind: "loading" }
  | { kind: "error"; error: ApiError }
  | { kind: "ready"; data: ReadClipResponse }

function SlugPage() {
  const { slug } = Route.useParams()
  const [state, setState] = useState<LoadState>({ kind: "loading" })
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    let cancelled = false
    setState({ kind: "loading" })

    readClip(slug)
      .then((data) => {
        if (!cancelled) {
          setState({ kind: "ready", data })
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setState({
            kind: "error",
            error: error instanceof ApiError ? error : new ApiError("Request failed", 0),
          })
        }
      })

    return () => {
      cancelled = true
    }
  }, [slug])

  async function handleCopy(source: string) {
    try {
      await navigator.clipboard.writeText(source)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  if (state.kind === "loading") {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (state.kind === "error") {
    const missing = state.error.status === 404
    return (
      <Alert variant={missing ? "default" : "destructive"}>
        <AlertTitle>{missing ? "Clip not found" : "Could not load clip"}</AlertTitle>
        <AlertDescription>
          {missing
            ? "This name has no live clip. It may never have existed or may have expired."
            : state.error.message}
        </AlertDescription>
      </Alert>
    )
  }

  const { data } = state

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-3xl font-semibold tracking-tight">{data.slug}</h1>
          <p className="text-sm text-muted-foreground">Expires {new Date(data.expiresAt).toLocaleString()}</p>
        </div>
        <Button type="button" variant="outline" onClick={() => handleCopy(data.body)}>
          {copied ? "Copied" : "Copy text"}
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Clip</CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="overflow-x-auto whitespace-pre-wrap break-words font-mono text-sm">{data.body}</pre>
        </CardContent>
      </Card>
    </div>
  )
}
