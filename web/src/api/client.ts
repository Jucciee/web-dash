import type { AuthResult, Credentials, Dashboard, Group, Layout, Service, ServiceInput, ServicePatch, SetupInput, Settings, SettingsPatch, ImportInput, ImportPreview, DockerScanInput, ImportCommit, ImportResult, WebStatus } from './types'

export class APIError extends Error {
  constructor(public status: number, public code: string, message: string, public requestId?: string) {
    super(message)
    this.name = 'APIError'
  }
}

let csrfToken: string | null = null
let csrfRequest: Promise<string> | null = null

async function getCSRF(): Promise<string> {
  if (csrfToken) return csrfToken
  csrfRequest ??= request<{ csrfToken: string }>('/auth/csrf').then(result => {
    csrfToken = result.csrfToken
    return result.csrfToken
  }).finally(() => { csrfRequest = null })
  return csrfRequest
}

async function request<T>(path: string, options: { method?: string; body?: unknown; revision?: number; idempotencyKey?: string } = {}): Promise<T> {
  const method = options.method ?? 'GET'
  const headers = new Headers({ Accept: 'application/json' })
  const multipart = options.body instanceof FormData
  if (options.body !== undefined && !multipart) headers.set('Content-Type', 'application/json')
  if (options.revision !== undefined) headers.set('If-Match', `"${options.revision}"`)
  if (options.idempotencyKey) headers.set('Idempotency-Key', options.idempotencyKey)
  if (method !== 'GET') headers.set('X-CSRF-Token', await getCSRF())
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      method, headers, credentials: 'same-origin',
      ...(options.body === undefined ? {} : { body: multipart ? options.body as FormData : JSON.stringify(options.body) }),
    })
  } catch {
    throw new APIError(0, 'NETWORK_ERROR', '서버에 연결하지 못했습니다. 서버 실행 상태를 확인하고 다시 시도해주세요.')
  }
  if (response.status === 204) return undefined as T
  const body = await response.json().catch(() => null)
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new Event('dashboard:unauthorized'))
    if (response.status === 403) csrfToken = null
    throw new APIError(response.status, body?.error?.code ?? 'REQUEST_FAILED', body?.error?.message ?? '요청을 처리하지 못했습니다. 다시 시도해주세요.', body?.meta?.requestId)
  }
  return body.data as T
}

async function authenticate(path: string, body: Credentials | SetupInput): Promise<AuthResult> {
  const result = await request<AuthResult>(path, { method: 'POST', body })
  csrfToken = result.csrfToken
  return result
}

export const api = {
  setupStatus: () => request<{ needsSetup: boolean }>('/setup/status'),
  me: async () => {
    try {
      const result = await request<AuthResult>('/me')
      csrfToken = result.csrfToken
      return result
    } catch (error) {
      if (error instanceof APIError && error.status === 401) return null
      throw error
    }
  },
  setup: (input: SetupInput) => authenticate('/setup', input),
  login: (input: Credentials) => authenticate('/sessions', input),
  logout: () => request<void>('/sessions/current', { method: 'DELETE' }).then(() => { csrfToken = null }),
  dashboards: () => request<Dashboard[]>('/dashboards'),
  dashboard: (id: string) => request<Dashboard>(`/dashboards/${encodeURIComponent(id)}`),
  webStatuses: (id: string) => request<WebStatus[]>(`/dashboards/${encodeURIComponent(id)}/web-statuses`),
  checkWebStatuses: (id: string) => request<WebStatus[]>(`/dashboards/${encodeURIComponent(id)}/web-statuses/check`, { method: 'POST' }),
  uploadIcon: (file: File) => { const body = new FormData(); body.set('file', file); return request<{ id: string }>('/icons', { method: 'POST', body }) },
  createService: (body: ServiceInput) => request<Service>('/services', { method: 'POST', body }),
  updateService: (service: Service, body: ServicePatch) => request<Service>(`/services/${service.id}`, { method: 'PATCH', body, revision: service.revision }),
  deleteService: (service: Service) => request<void>(`/services/${service.id}`, { method: 'DELETE', revision: service.revision }),
  createGroup: (dashboardId: string, name: string) => request<Group>(`/dashboards/${dashboardId}/groups`, { method: 'POST', body: { name } }),
  updateGroup: (group: Group, name: string) => request<Group>(`/groups/${group.id}`, { method: 'PATCH', body: { name }, revision: group.revision }),
  deleteGroup: (group: Group) => request<void>(`/groups/${group.id}`, { method: 'DELETE', revision: group.revision }),
  saveLayout: (dashboard: Dashboard, body: Layout) => request<Dashboard>(`/dashboards/${dashboard.id}/layout`, { method: 'PUT', body, revision: dashboard.revision }),
  settings: () => request<Settings>('/settings'),
  updateSettings: (body: SettingsPatch, revision: number) => request<Settings>('/settings', { method: 'PATCH', body, revision }),
  importPreview: (body: ImportInput) => request<ImportPreview>('/imports/preview', { method: 'POST', body }),
  scanDocker: (body: DockerScanInput) => request<ImportPreview>('/imports/docker/scan', { method: 'POST', body }),
  commitImport: (id: string, body: ImportCommit, idempotencyKey: string) => request<ImportResult>(`/imports/${id}/commit`, { method: 'POST', body, idempotencyKey }),
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '요청을 처리하지 못했습니다. 다시 시도해주세요.'
}
