import type { AddonField, AddonManifest } from '../../api/types'

export const newField = (): AddonField => ({ label: '값', path: '', format: 'text', unit: '', scale: 1, decimals: 0, valueMap: {} })
export const newManifest = (): AddonManifest => ({ schemaVersion: 1, name: '', icon: 'server', display: 'stats', refreshSeconds: 30, request: { method: 'GET', url: '', headers: [], body: '' }, inputs: [], fields: [newField()] })

export function parseManifest(source: string): AddonManifest {
  if (new TextEncoder().encode(source).length > 48 * 1024) throw new Error('애드온 설정 파일은 48KiB까지 가져올 수 있습니다.')
  const value = JSON.parse(source)
  const object = (item: unknown): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item)
  if (!object(value) || value.schemaVersion !== 1 || typeof value.name !== 'string' || !object(value.request) || typeof value.request.url !== 'string' || !Array.isArray(value.fields) || !value.fields.length) throw new Error('web-dash schemaVersion 1 애드온 JSON을 선택해주세요.')
  if (value.fields.length > 16 || (value.inputs !== undefined && (!Array.isArray(value.inputs) || value.inputs.length > 16))) throw new Error('입력 항목과 표시할 값은 16개까지 사용할 수 있습니다.')
  const strings = (item: Record<string, unknown>, keys: string[]) => keys.every(key => item[key] === undefined || typeof item[key] === 'string')
  if (!strings(value, ['icon', 'display']) || (value.display !== undefined && !['stats', 'list'].includes(String(value.display))) || (value.refreshSeconds !== undefined && (typeof value.refreshSeconds !== 'number' || ![0, 10, 30, 60, 300].includes(value.refreshSeconds))) || !strings(value.request, ['method', 'body']) || (value.request.method !== undefined && !['GET', 'POST'].includes(String(value.request.method)))) throw new Error('애드온 표시 방식과 요청 설정을 확인해주세요.')
  if (value.request.headers !== undefined && (!Array.isArray(value.request.headers) || value.request.headers.length > 16 || value.request.headers.some(header => !object(header) || typeof header.name !== 'string' || typeof header.value !== 'string'))) throw new Error('요청 헤더는 name과 value로 구성한 배열이어야 합니다.')
  return {
    ...newManifest(), ...value,
    request: { method: 'GET', headers: [], body: '', ...value.request, url: value.request.url },
    inputs: ((value.inputs ?? []) as unknown[]).map(input => {
      if (!object(input) || typeof input.key !== 'string' || !/^[A-Za-z][A-Za-z0-9_]{0,39}$/.test(input.key) || ['constructor', 'prototype'].includes(input.key)) throw new Error('입력 항목의 key를 확인해주세요.')
      if (!strings(input, ['label', 'type', 'default']) || (input.required !== undefined && typeof input.required !== 'boolean') || (input.type !== undefined && !['text', 'secret'].includes(String(input.type)))) throw new Error('입력 항목의 이름·종류·기본값을 확인해주세요.')
      if (input.type === 'secret' && input.default) throw new Error('보안 입력에는 기본값을 넣지 마세요. 내 연결값에 입력해주세요.')
      return { label: input.key, type: 'text', required: false, default: '', ...input, key: input.key }
    }),
    fields: value.fields.map(field => {
      if (!object(field) || typeof field.label !== 'string' || typeof field.path !== 'string') throw new Error('표시 항목의 label과 path를 확인해주세요.')
      if (!strings(field, ['format', 'unit']) || (field.scale !== undefined && (typeof field.scale !== 'number' || !Number.isFinite(field.scale))) || (field.decimals !== undefined && (typeof field.decimals !== 'number' || !Number.isInteger(field.decimals))) || (field.valueMap !== undefined && (!object(field.valueMap) || Object.values(field.valueMap).some(value => typeof value !== 'string')))) throw new Error('표시 항목의 형식·계산 설정을 확인해주세요.')
      return { ...newField(), ...field }
    }),
  }
}

export function exportManifest(manifest: AddonManifest) {
  const blob = new Blob([JSON.stringify(manifest, null, 2) + '\n'], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${manifest.name.replace(/[^\p{L}\p{N}_-]+/gu, '-').slice(0, 60) || 'addon'}.web-dash.json`
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}
