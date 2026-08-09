const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new ApiError(res.status, text)
  }

  return res.json() as Promise<T>
}

export function get<T>(url: string): Promise<T> {
  return request<T>('GET', url)
}

export function post<T>(url: string, body?: unknown): Promise<T> {
  return request<T>('POST', url, body)
}

export function put<T>(url: string, body?: unknown): Promise<T> {
  return request<T>('PUT', url, body)
}

export function patch<T>(url: string, body?: unknown): Promise<T> {
  return request<T>('PATCH', url, body)
}

export function del<T>(url: string): Promise<T> {
  return request<T>('DELETE', url)
}
