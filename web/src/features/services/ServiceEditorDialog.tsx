import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, APIError } from '../../api/client'
import type { Group, Service, ServiceInput } from '../../api/types'
import { Button } from '../../components/Button'
import { Dialog } from '../../components/Dialog'
import { ErrorNotice } from '../../components/Feedback'
import { iconOptions, ServiceIcon } from '../dashboard/ServiceIcon'

export function ServiceEditorDialog({ service, groups, defaultGroupId, onClose }: { service?: Service; groups: Group[]; defaultGroupId?: string; onClose: () => void }) {
  const [input, setInput] = useState<ServiceInput>({
    groupId: service?.groupId ?? defaultGroupId ?? groups[0]?.id ?? '',
    name: service?.name ?? '', url: service?.url ?? '', iconKey: (service?.iconKey || 'server') as ServiceInput['iconKey'],
    description: service?.description ?? '', hidden: service?.hidden ?? false,
    iconMode: service?.iconMode ?? 'site', iconUrl: service?.iconUrl ?? '', iconAssetId: service?.iconAssetId ?? '', webCheckEnabled: service?.webCheckEnabled ?? true,
  })
  const [validation, setValidation] = useState<string | null>(null)
  const upload = useMutation({ mutationFn: api.uploadIcon, onSuccess: result => { update('iconAssetId', result.id); setValidation(null) } })
  const qc = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => service ? api.updateService(service, input) : api.createService(input),
    onSuccess: async () => { await qc.invalidateQueries({ queryKey: ['dashboards'] }); onClose() },
    onError: error => { if (error instanceof APIError && error.status === 409) void qc.invalidateQueries({ queryKey: ['dashboards'] }) },
  })
  function update<K extends keyof ServiceInput>(key: K, value: ServiceInput[K]) { setInput(current => ({ ...current, [key]: value })) }
  const busy = mutation.isPending || upload.isPending
  function chooseFile(file?: File) {
    if (!file) return
    if (file.size > 1024 * 1024) { setValidation('아이콘 파일은 1MB까지 업로드할 수 있습니다.'); return }
    upload.mutate(file)
  }
  function submit(event: FormEvent) {
    event.preventDefault()
    try {
      const url = new URL(input.url.trim())
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || /\s/.test(input.url)) throw new Error()
    } catch { setValidation('로그인 정보가 없는 http 또는 https 주소를 입력해주세요.'); return }
    if (!input.name.trim()) { setValidation('링크 이름을 입력해주세요.'); return }
    if (input.iconMode === 'upload' && !input.iconAssetId) { setValidation('아이콘 파일을 선택해주세요.'); return }
    setValidation(null)
    mutation.mutate()
  }
  return <Dialog title={service ? '링크 편집' : '링크 추가'} onClose={onClose} busy={busy}>
    <form className="form-stack" onSubmit={submit}>
      <p className="helper">홈서버 앱과 일반 웹사이트를 링크로 추가할 수 있습니다.</p>
      <label className="field">링크 이름<input autoFocus name="name" required maxLength={80} value={input.name} onChange={event => update('name', event.target.value)} placeholder="예: 사진 서버" disabled={busy} /></label>
      <div className="field"><label htmlFor="service-url">접속 주소</label><input id="service-url" name="url" type="url" required maxLength={2048} value={input.url} onChange={event => update('url', event.target.value)} placeholder="https://photos.example.com" disabled={busy} aria-describedby="url-help" /><small id="url-help">브라우저에서 실제로 접속하는 주소를 입력하세요.</small></div>
      <div className="form-row"><div className="field"><label htmlFor="service-group">그룹</label><select id="service-group" name="groupId" required value={input.groupId} onChange={event => update('groupId', event.target.value)} disabled={busy}>{groups.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</select></div><div className="field"><label htmlFor="icon-mode">아이콘 표시</label><select id="icon-mode" value={input.iconMode} onChange={event => { update('iconMode', event.target.value as ServiceInput['iconMode']); upload.reset() }} disabled={busy}><option value="site">사이트 아이콘 자동</option><option value="url">이미지 주소</option><option value="upload">파일 업로드</option><option value="builtin">기본 아이콘</option></select></div></div>
      {input.iconMode === 'site' && <p className="helper">저장 후 사이트의 아이콘을 가져옵니다. 찾지 못하면 기본 아이콘을 표시합니다.</p>}
      {input.iconMode === 'url' && <div className="field"><label htmlFor="icon-url">아이콘 이미지 주소</label><input id="icon-url" type="url" required maxLength={2048} placeholder="https://example.com/icon.png" value={input.iconUrl} onChange={event => update('iconUrl', event.target.value)} disabled={busy} /><small>PNG, JPG, WebP, ICO 이미지를 지원합니다.</small></div>}
      {input.iconMode === 'upload' && <><div className="field"><label htmlFor="icon-file">아이콘 파일</label><input id="icon-file" type="file" accept=".png,.jpg,.jpeg,.webp,.ico" onChange={event => chooseFile(event.target.files?.[0])} disabled={busy} /><small>PNG, JPG, WebP, ICO · 최대 1MB · 최대 2048px</small></div>{upload.isPending && <p className="helper" role="status">아이콘 업로드 중…</p>}{input.iconAssetId && <div className="icon-preview"><img src={`/api/v1/icons/${input.iconAssetId}`} alt="업로드한 아이콘 미리보기" /><span className="helper" role="status">업로드 완료</span></div>}{upload.isError && <ErrorNotice error={upload.error} />}</>}
      {input.iconMode === 'builtin' && <div className="field"><label htmlFor="service-icon">기본 아이콘</label><select id="service-icon" value={input.iconKey} onChange={event => update('iconKey', event.target.value as ServiceInput['iconKey'])} disabled={busy}>{iconOptions.map(item => <option key={item.key} value={item.key}>{item.label}</option>)}</select><span><ServiceIcon iconKey={input.iconKey} /></span></div>}
      <label className="field">설명 <span className="optional">선택</span><textarea name="description" maxLength={500} rows={3} value={input.description} onChange={event => update('description', event.target.value)} disabled={busy} placeholder="이 링크에 대한 짧은 메모" /></label>
      <label className="checkbox-field"><input type="checkbox" checked={input.hidden} onChange={event => update('hidden', event.target.checked)} disabled={busy} />대시보드에서 숨기기</label>
      <div><label className="checkbox-field"><input type="checkbox" checked={input.webCheckEnabled} onChange={event => update('webCheckEnabled', event.target.checked)} disabled={busy} />웹 상태 확인</label><p className="helper">페이지에 접속하면 web-dash 서버가 HTTP 응답을 확인합니다. 즐겨찾기로만 쓸 링크는 끌 수 있습니다.</p></div>
      {(validation || mutation.isError) && <ErrorNotice error={validation ? new Error(validation) : mutation.error} />}
      <div className="dialog-actions"><Button onClick={onClose} disabled={busy}>취소</Button><Button type="submit" variant="primary" loading={mutation.isPending} disabled={busy || (input.iconMode === 'upload' && !input.iconAssetId)}>{mutation.isPending ? '저장하는 중…' : service ? '변경 저장' : '링크 추가'}</Button></div>
    </form>
  </Dialog>
}
