import { useEffect, useState } from "react"
import { ApiError, fetchHealth, type HealthResponse } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"

type LoadState =
  | { kind: "loading" }
  | { kind: "error"; error: ApiError }
  | { kind: "ready"; data: HealthResponse }

export function StatusView() {
  const [state, setState] = useState<LoadState>({ kind: "loading" })

  useEffect(() => {
    let cancelled = false

    fetchHealth()
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
  }, [])

  if (state.kind === "loading") {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    )
  }

  if (state.kind === "error") {
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load status</AlertTitle>
        <AlertDescription>{state.error.message}</AlertDescription>
      </Alert>
    )
  }

  const { data } = state
  const healthy = data.status === "ok"
  const migrations = data.migrations

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">Stack status</h1>
        <p className="text-muted-foreground">
          Live diagnostics from the Go API and Postgres through the development proxy.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Overall</CardTitle>
          <CardDescription>Server process health</CardDescription>
        </CardHeader>
        <CardContent>
          <Badge variant={healthy ? "default" : "destructive"}>
            {healthy ? "Healthy" : "Degraded"}
          </Badge>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Database</CardTitle>
          <CardDescription>Connection pool reachability</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {data.database.reachable ? (
            <>
              <Badge>Reachable</Badge>
              <p className="text-sm text-muted-foreground">
                Round-trip latency: {data.database.latencyMs ?? "unknown"} ms
              </p>
              {data.serverTime ? (
                <p className="text-sm text-muted-foreground">Server time: {data.serverTime}</p>
              ) : null}
            </>
          ) : (
            <Alert variant="destructive">
              <AlertTitle>Database unreachable</AlertTitle>
              <AlertDescription>
                Postgres is not accepting connections. Start the local service and confirm
                `DATABASE_URL`.
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Migrations</CardTitle>
          <CardDescription>Schema version reported by goose</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {migrations === undefined ? (
            <Alert variant="destructive">
              <AlertTitle>Migration state unknown</AlertTitle>
              <AlertDescription>
                The server could not run the pending-migration check, so the schema version
                is unknown rather than current. Restore the database connection and reload.
              </AlertDescription>
            </Alert>
          ) : migrations.pending ? (
            <Alert>
              <AlertTitle>Migrations pending</AlertTitle>
              <AlertDescription>
                Run `make migrate` from the repository root before relying on generated queries.
              </AlertDescription>
            </Alert>
          ) : (
            <p className="text-sm">
              No pending migrations. Current version: {migrations.currentVersion}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
