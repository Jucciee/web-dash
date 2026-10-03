import { readFile, writeFile } from 'node:fs/promises'

const target = new URL('../docs/openapi.json', import.meta.url)
const spec = JSON.parse(await readFile(target, 'utf8'))
const schemas = spec.components.schemas
const ref = name => ({ $ref: `#/components/schemas/${name}` })
const string = (extra = {}) => ({ type: 'string', ...extra })
const uuid = () => string({ format: 'uuid' })
const integer = () => ({ type: 'integer', minimum: 1 })
const array = items => ({ type: 'array', items })
const object = (properties, required = Object.keys(properties)) => ({ type: 'object', properties, required, additionalProperties: false })
const linkFields = { iconMode: string({ enum: ['builtin', 'site', 'url', 'upload'] }), iconUrl: string({ maxLength: 2048 }), iconAssetId: string({ maxLength: 64 }), webCheckEnabled: { type: 'boolean', default: true } }
Object.assign(schemas.ServiceInput.properties, linkFields)
Object.assign(schemas.ServicePatch.properties, linkFields)
Object.assign(schemas.Service.properties, linkFields)
schemas.Service.required = [...new Set([...schemas.Service.required, ...Object.keys(linkFields)])]
schemas.WebStatus = object({ serviceId: uuid(), state: string({ enum: ['unknown', 'checking', 'disabled', 'online', 'auth', 'error', 'offline', 'timeout'] }), checking: { type: 'boolean' }, httpStatus: { type: 'integer', minimum: 0 }, latencyMs: { type: 'integer', minimum: 0 }, checkedAt: { type: ['string', 'null'], format: 'date-time' }, message: string() })
schemas.WebStatusesEnvelope = object({ data: array(ref('WebStatus')), meta: ref('Meta') })
schemas.IconUpload = object({ id: string({ pattern: '^[a-f0-9]{64}$' }) })
schemas.IconUploadEnvelope = object({ data: ref('IconUpload'), meta: ref('Meta') })
schemas.Settings = object({ title: string({ minLength: 1, maxLength: 80 }), subtitle: string({ maxLength: 200 }), theme: string({ enum: ['light', 'dark', 'system'] }), accent: string({ enum: ['blue', 'green', 'amber', 'slate'] }), compact: { type: 'boolean' }, customCss: string({ description: 'UTF-8 maximum 32KiB. Applied only on home.' }), revision: integer() })
const { revision: _revision, ...settingsFields } = schemas.Settings.properties
schemas.SettingsPatch = object(settingsFields, [])
schemas.ImportFile = object({ path: string({ maxLength: 240, description: 'Unique relative virtual path, never a server filesystem path' }), content: string({ description: 'UTF-8 maximum 2MiB; all files combined maximum 10MiB' }) })
schemas.ImportInput = object({ dashboardId: uuid(), kind: string({ enum: ['nginx', 'caddy'] }), sourceName: string({ maxLength: 80 }), entryPath: string({ maxLength: 240 }), files: { ...array(ref('ImportFile')), minItems: 1, maxItems: 50 } }, ['dashboardId', 'kind', 'entryPath', 'files'])
schemas.DockerScanInput = object({ dashboardId: uuid(), folder: string({ maxLength: 2048, description: 'Absolute folder on Dashboard server. Scan skips symbolic links, hidden directories and node_modules, up to four subdirectory levels.' }), publicHost: string({ maxLength: 253, description: 'Host or IP used for wildcard-bound runtime published ports, without a port or scheme' }) })
schemas.ImportCandidate = object({ id: uuid(), name: string(), url: string({ description: 'Concrete HTTP/HTTPS URL. Unresolved entries are excluded and reported in preview warnings.' }), file: string(), line: { type: 'integer', minimum: 0 }, upstreams: array(string()), warnings: array(string()), diff: string({ enum: ['new', 'duplicate'] }), existingServiceId: string() })
schemas.ImportPreview = object({ id: uuid(), dashboardId: uuid(), kind: string({ enum: ['nginx', 'caddy', 'docker'] }), sourceName: string(), dashboardRevision: integer(), expiresAt: string({ format: 'date-time' }), candidates: array(ref('ImportCandidate')), warnings: array(string()) })
schemas.ImportSelection = object({ candidateId: uuid(), ...schemas.ServiceInput.properties }, ['candidateId', ...schemas.ServiceInput.required])
schemas.ImportCommit = object({ dashboardRevision: integer(), items: { ...array(ref('ImportSelection')), minItems: 1, maxItems: 500 } })
schemas.ImportResult = object({ serviceIds: array(uuid()), dashboardRevision: integer() })
for (const name of ['Settings', 'ImportPreview', 'ImportResult']) schemas[`${name}Envelope`] = object({ data: ref(name), meta: ref('Meta') })
const csrf = structuredClone(spec.paths['/services'].post.parameters[0])
const match = structuredClone(spec.paths['/services/{id}'].patch.parameters[2])
const id = { name: 'id', in: 'path', required: true, schema: uuid() }
const error = { description: 'API error: 400, 401, 403, 404, 409, 413, 422, 500 or 502', content: { 'application/json': { schema: ref('ErrorEnvelope') } } }
const operation = (summary, result, status = 200, input, parameters = []) => ({ summary, parameters, ...(input ? { requestBody: { required: true, content: { 'application/json': { schema: ref(input) } } } } : {}), responses: { [status]: { description: 'Success', content: { 'application/json': { schema: ref(`${result}Envelope`) } } }, default: error } })
spec.paths['/settings'] = { get: operation('Read persisted appearance settings', 'Settings'), patch: operation('Update appearance with optimistic revision check', 'Settings', 200, 'SettingsPatch', [csrf, match]) }
spec.paths['/settings/custom.css'] = { get: { summary: 'Read authenticated custom CSS for home', responses: { 200: { description: 'CSS stylesheet, no-store', content: { 'text/css': { schema: string() } } }, default: error } } }
spec.paths['/imports/preview'] = { post: operation('Preview uploaded Nginx/Caddy files (JSON virtual file bundle)', 'ImportPreview', 201, 'ImportInput', [csrf]) }
spec.paths['/imports/docker/scan'] = { post: operation('Find Compose files and runtime published TCP ports from running containers', 'ImportPreview', 201, 'DockerScanInput', [csrf]) }
spec.paths['/imports/{id}'] = { get: operation('Read owned, unexpired preview', 'ImportPreview', 200, undefined, [id]) }
spec.paths['/imports/{id}/commit'] = { post: operation('Atomically create selected cards; duplicate-safe idempotent replay', 'ImportResult', 200, 'ImportCommit', [id, csrf, { name: 'Idempotency-Key', in: 'header', required: true, schema: uuid() }]) }
spec.paths['/dashboards/{id}/web-statuses'] = { get: { summary: 'Read cached server-side HTTP observations; queue missing or stale checks', parameters: [id], responses: { 200: { description: 'Success', content: { 'application/json': { schema: ref('WebStatusesEnvelope') } } }, default: error } } }
spec.paths['/dashboards/{id}/web-statuses/check'] = { post: { summary: 'Queue manual HTTP checks; coalesced with a 5s minimum interval', parameters: [id, csrf], responses: { 200: { description: 'Current observations with queued checks', content: { 'application/json': { schema: ref('WebStatusesEnvelope') } } }, default: error } } }
spec.paths['/icons'] = { post: { summary: 'Upload a validated PNG/JPEG/WebP/ICO icon, up to 1MiB and 2048px', parameters: [csrf], requestBody: { required: true, content: { 'multipart/form-data': { schema: object({ file: string({ format: 'binary' }) }) } } }, responses: { 201: { description: 'Stored image ID', content: { 'application/json': { schema: ref('IconUploadEnvelope') } } }, default: error } } }
for (const path of ['/icons/{id}', '/services/{id}/icon']) spec.paths[path] = { get: { summary: 'Read an authenticated, validated icon image; remote discovery is cached and bounded', parameters: [{ ...id, schema: string() }], responses: { 200: { description: 'Raster icon image', content: { 'image/png': { schema: string({ format: 'binary' }) }, 'image/jpeg': { schema: string({ format: 'binary' }) }, 'image/webp': { schema: string({ format: 'binary' }) }, 'image/x-icon': { schema: string({ format: 'binary' }) } } }, 204: { description: 'No site icon found; use built-in fallback' }, default: error } } }
spec.info.version = '0.3.0'
spec.info.description = 'Implemented authentication, link/group management, themes/custom CSS, site/custom/uploaded icons, server-side periodic HTTP status checks, Nginx/Caddy import, and runtime Docker port discovery. Docker/host metrics and backups remain planned.'
await writeFile(target, JSON.stringify(spec, null, 2) + '\n')
