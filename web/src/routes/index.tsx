import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { useEffect, useState } from "react"
import {
  ApiError,
  checkAvailability,
  createClip,
  DEFAULT_TTL_SECONDS,
  TTL_PRESETS,
  type TtlSeconds,
} from "@/lib/api"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

export const Route = createFileRoute("/")({
  component: IndexPage,
})

function IndexPage() {
  const navigate = useNavigate()
  const [slug, setSlug] = useState("")
  const [body, setBody] = useState("")
  const [ttlSeconds, setTtlSeconds] = useState<TtlSeconds>(DEFAULT_TTL_SECONDS)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [availabilityHint, setAvailabilityHint] = useState<string | null>(null)

  useEffect(() => {
    const trimmed = slug.trim()
    if (trimmed.length < 3) {
      setAvailabilityHint(null)
      return
    }

    setAvailabilityHint(null)

    let cancelled = false
    const timer = window.setTimeout(() => {
      checkAvailability(trimmed)
        .then((result) => {
          if (cancelled) {
            return
          }
          if (result.available) {
            setAvailabilityHint("This name looks free right now — advisory only, not a reservation.")
          } else {
            setAvailabilityHint("This name looks taken right now — you can still try to claim it.")
          }
        })
        .catch((err: unknown) => {
          if (cancelled) {
            return
          }
          if (err instanceof ApiError) {
            setAvailabilityHint(err.message)
          } else {
            setAvailabilityHint(null)
          }
        })
    }, 400)

    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [slug])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const result = await createClip(slug, body, ttlSeconds)
      await navigate({ to: "/$slug", params: { slug: result.slug } })
    } catch (err) {
      if (err instanceof ApiError && err.code === "slug_in_use") {
        setError("that name is in use right now")
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError("Could not create clip")
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">Paste a clip</h1>
        <p className="text-muted-foreground">
          Choose a name, pick how long it lives, and paste text. Anyone who knows the name can read it
          until it expires.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>New clip</CardTitle>
          <CardDescription>Names are case-sensitive and must be 3–64 characters.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="slug">Name</Label>
              <Input
                id="slug"
                name="slug"
                autoComplete="off"
                spellCheck={false}
                required
                value={slug}
                onChange={(event) => setSlug(event.target.value)}
              />
              <p className="text-sm text-muted-foreground">
                Guessable names are like a bulletin board; hard-to-guess names are like a secret link.
                This name becomes part of the URL.
              </p>
              {availabilityHint ? (
                <p className="text-sm text-muted-foreground">{availabilityHint}</p>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label htmlFor="ttl">Lifetime (anonymous limit)</Label>
              <select
                id="ttl"
                name="ttl"
                className="border-input bg-background ring-offset-background focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
                value={ttlSeconds}
                onChange={(event) => setTtlSeconds(Number(event.target.value) as TtlSeconds)}
              >
                {TTL_PRESETS.map((preset) => (
                  <option key={preset.seconds} value={preset.seconds}>
                    {preset.label} — {preset.description}
                  </option>
                ))}
              </select>
              <p className="text-sm text-muted-foreground">
                This is the anonymous limit. Longer lifetimes unlock when you sign in later.
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="body">Text</Label>
              <Textarea
                id="body"
                name="body"
                required
                rows={12}
                value={body}
                onChange={(event) => setBody(event.target.value)}
              />
            </div>

            {error ? (
              <Alert variant="destructive">
                <AlertTitle>Could not create clip</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}

            <Button type="submit" disabled={submitting}>
              {submitting ? "Creating…" : "Create clip"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
