export type { components } from "@/lib/api.gen"

import type { components } from "@/lib/api.gen"

export type HealthResponse = components["schemas"]["HealthResponse"]
export type CreateClipResponse = components["schemas"]["CreateClipResponse"]
export type ReadClipResponse = components["schemas"]["ReadClipResponse"]
export type ClipAvailabilityResponse = components["schemas"]["ClipAvailabilityResponse"]

export type TtlSeconds = NonNullable<components["schemas"]["CreateClipRequest"]["ttlSeconds"]>

export const TTL_PRESETS: { label: string; seconds: TtlSeconds; description: string }[] = [
  { label: "10 minutes", seconds: 600, description: "Good for passwords and one-time secrets" },
  { label: "1 hour", seconds: 3600, description: "Short-lived notes and links" },
  { label: "2 hours", seconds: 7200, description: "The anonymous limit — longest without signing in" },
]

export const DEFAULT_TTL_SECONDS: TtlSeconds = 7200

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

export async function createClip(
  slug: string,
  body: string,
  ttlSeconds: TtlSeconds = DEFAULT_TTL_SECONDS,
): Promise<CreateClipResponse> {
  const response = await fetch(apiUrl("/clips"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ slug, body, ttlSeconds }),
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

export async function checkAvailability(slug: string): Promise<ClipAvailabilityResponse> {
  const response = await fetch(apiUrl(`/clips/${encodeURIComponent(slug)}/availability`))
  const payload = await parseJsonResponse(response)
  throwIfError(response, payload)
  return payload as ClipAvailabilityResponse
}
