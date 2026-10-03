/** A structural problem reported by the server when saving a workflow. */
export type Problem = {
  message: string
  step?: string
  edge?: { from: string; to: string }
}

export class ApiError extends Error {
  readonly status: number
  readonly problems: Problem[]

  constructor(status: number, message: string, problems: Problem[] = []) {
    super(message)
    this.status = status
    this.problems = problems
  }
}

async function request<T>(method: string, path: string, body?: unknown, init?: RequestInit) {
  const res = await fetch(`/api${path}`, {
    ...init,
    method,
    headers: {
      Accept: 'application/json',
      ...(body !== undefined && { 'Content-Type': 'application/json' }),
      ...init?.headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    const data = (await res.json().catch(() => null)) as {
      error?: string
      problems?: Problem[]
    } | null
    throw new ApiError(res.status, data?.error ?? res.statusText, data?.problems)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string, init?: RequestInit) => request<T>('GET', path, undefined, init),
  post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
  delete: (path: string) => request<void>('DELETE', path),
}
