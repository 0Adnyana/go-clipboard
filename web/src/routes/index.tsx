import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { useState } from "react"
import { ApiError, createClip } from "@/lib/api"
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
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const result = await createClip(slug, body)
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
          Choose a name and paste text. Anyone who knows the name can read it for the next two hours.
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
                This name becomes part of the URL. Anyone with the link can read the clip until it
                expires.
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
