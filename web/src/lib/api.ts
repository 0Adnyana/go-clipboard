export type HealthResponse = {
  status: string
  database: {
    reachable: boolean
    latencyMs?: number
  }
  // Absent when the server could not run the pending-migration check.
  migrations?: {
    pending: boolean
    currentVersion: number
  }
  serverTime?: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
    this.code = code
  }
}

function apiUrl(path: string): string {
  const normalized = path.startsWith("/") ? path : `/${path}`
  return `/api${normalized}`
}

export async function fetchHealth(): Promise<HealthResponse> {
  const response = await fetch(apiUrl("/health"))

  let body: unknown
  try {
    body = await response.json()
  } catch {
    throw new ApiError("Response body was not valid JSON", response.status)
  }

  if (!response.ok) {
    const errorBody = body as { code?: string; message?: string }
    throw new ApiError(
      errorBody.message ?? "Request failed",
      response.status,
      errorBody.code,
    )
  }

  return body as HealthResponse
}
