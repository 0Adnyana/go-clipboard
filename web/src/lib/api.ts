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

export type CreateClipResponse = {
  slug: string
  expiresAt: string
}

export type ReadClipResponse = {
  slug: string
  body: string
  expiresAt: string
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

async function parseJsonResponse(response: Response): Promise<unknown> {
  try {
    return await response.json()
  } catch {
    throw new ApiError("Response body was not valid JSON", response.status)
  }
}

function throwIfError(response: Response, body: unknown): void {
  if (response.ok) {
    return
  }
  const errorBody = body as { code?: string; message?: string }
  throw new ApiError(
    errorBody.message ?? "Request failed",
    response.status,
    errorBody.code,
  )
}

export async function fetchHealth(): Promise<HealthResponse> {
  const response = await fetch(apiUrl("/health"))
  const body = await parseJsonResponse(response)
  throwIfError(response, body)
  return body as HealthResponse
}

export async function createClip(slug: string, body: string): Promise<CreateClipResponse> {
  const response = await fetch(apiUrl("/clips"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ slug, body }),
  })
  const payload = await parseJsonResponse(response)
  throwIfError(response, payload)
  return payload as CreateClipResponse
}

export async function readClip(slug: string): Promise<ReadClipResponse> {
  const response = await fetch(apiUrl(`/clips/${encodeURIComponent(slug)}`))
  const payload = await parseJsonResponse(response)
  throwIfError(response, payload)
  return payload as ReadClipResponse
}
