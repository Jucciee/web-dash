import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { Group } from '../../api/types'
import { api } from '../../api/client'
import { Button } from '../../components/Button'
import { Dialog } from '../../components/Dialog'
import { ErrorNotice } from '../../components/Feedback'

export function GroupEditorDialog({ dashboardId, group, onClose }: { dashboardId: string; group?: Group; onClose: () => void }) {
  const [name, setName] = useState(group?.name ?? '')
  const qc = useQueryClient()
  const save = useMutation({ mutationFn: () => group ? api.updateGroup(group, name.trim()) : api.createGroup(dashboardId, name.trim()), onSuccess: async () => { await qc.invalidateQueries({ queryKey: ['dashboards'] }); onClose() } })
  const remove = useMutation({ mutationFn: () => api.deleteGroup(group!), onSuccess: async () => { await qc.invalidateQueries({ queryKey: ['dashboards'] }); onClose() } })
  const busy = save.isPending || remove.isPending
  function submit(event: FormEvent) { event.preventDefault(); if (name.trim()) save.mutate() }
  return <Dialog title={group ? '그룹 편집' : '그룹 추가'} onClose={onClose} busy={busy}>
    <form className="form-stack" onSubmit={submit}><label className="field">그룹 이름<input autoFocus name="groupName" value={name} onChange={event => setName(event.target.value)} required maxLength={80} placeholder="예: 미디어" disabled={busy} /></label>{(save.isError || remove.isError) && <ErrorNotice error={save.error ?? remove.error} />}
      {group && <p className="helper">{group.services.length ? '그룹을 삭제하려면 링크를 먼저 이동하거나 삭제하세요.' : '빈 그룹은 삭제할 수 있습니다.'}</p>}
      <div className="dialog-actions">{group && <Button variant="danger" disabled={busy || group.services.length > 0} onClick={() => remove.mutate()} loading={remove.isPending}>그룹 삭제</Button>}<Button onClick={onClose} disabled={busy}>취소</Button><Button type="submit" variant="primary" loading={save.isPending} disabled={busy || !name.trim()}>저장</Button></div>
    </form>
  </Dialog>
}
