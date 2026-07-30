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
  | { kind: "ready"; data: ReadClipResponse; skewMs: number }
  | { kind: "expired" }

function formatRemaining(ms: number): string {
  if (ms <= 0) {
    return "0:00"
  }
  const totalSeconds = Math.ceil(ms / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
  }
  return `${minutes}:${String(seconds).padStart(2, "0")}`
}

function SlugPage() {
  const { slug } = Route.useParams()
  const [state, setState] = useState<LoadState>({ kind: "loading" })
  const [remainingMs, setRemainingMs] = useState<number | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    let cancelled = false
    setState({ kind: "loading" })
    setRemainingMs(null)

    readClip(slug)
      .then((data) => {
        if (cancelled) {
          return
        }
        const serverNow = Date.parse(data.serverTime)
        const expiresAt = Date.parse(data.expiresAt)
        const skewMs = serverNow - Date.now()
        const remaining = expiresAt - (Date.now() + skewMs)
        if (remaining <= 0) {
          setState({ kind: "expired" })
          return
        }
        setState({ kind: "ready", data, skewMs })
        setRemainingMs(remaining)
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          const apiError = error instanceof ApiError ? error : new ApiError("Request failed", 0)
          if (apiError.status === 404) {
            setState({ kind: "expired" })
            return
          }
          setState({ kind: "error", error: apiError })
        }
      })

    return () => {
      cancelled = true
    }
  }, [slug])

  useEffect(() => {
    if (state.kind !== "ready") {
      return
    }

    let cancelled = false
    const expiresAt = Date.parse(state.data.expiresAt)
    const tick = () => {
      if (cancelled) {
        return
      }
      const remaining = expiresAt - (Date.now() + state.skewMs)
      if (remaining <= 0) {
        if (!cancelled) {
          setState({ kind: "expired" })
          setRemainingMs(0)
          void readClip(slug).catch(() => {
            if (!cancelled) {
              setState({ kind: "expired" })
            }
          })
        }
        return
      }
      if (!cancelled) {
        setRemainingMs(remaining)
      }
    }

    tick()
    const id = window.setInterval(tick, 1000)
    return () => {
      cancelled = true
      window.clearInterval(id)
    }
  }, [slug, state])

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

  if (state.kind === "expired") {
    return (
      <Alert>
        <AlertTitle>This clip has expired</AlertTitle>
        <AlertDescription>
          The countdown reached zero or this name has no live clip anymore. Create a new one if you still
          need to share something.
        </AlertDescription>
      </Alert>
    )
  }

  if (state.kind === "error") {
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load clip</AlertTitle>
        <AlertDescription>{state.error.message}</AlertDescription>
      </Alert>
    )
  }

  const { data } = state

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-3xl font-semibold tracking-tight">{data.slug}</h1>
          <p className="text-sm text-muted-foreground">
            Expires in {formatRemaining(remainingMs ?? 0)} (server clock)
          </p>
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
