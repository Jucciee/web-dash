import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowDownToLine, ArrowRight, LayoutGrid, Save } from 'lucide-react'
import { api, APIError } from '../../api/client'
import type { Settings, SettingsPatch } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import { useSettings } from './useSettings'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const query = useSettings()
  if (query.isPending) return <p role="status" className="helper">설정을 불러오는 중…</p>
  if (query.isError) return <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
  return <div className={styles.page}>
    <header className={styles.heading}><p className="breadcrumb">내 홈서버 / 설정</p><h1>내 공간을 내 방식으로</h1><p>링크 관리와 화면 설정을 한곳에 모았습니다.</p></header>
    <nav className={styles.shortcuts} aria-label="설정 바로가기"><Link to="/settings/services"><LayoutGrid size={20} /><span><strong>링크 관리</strong><small>카드 · 그룹 · 표시 순서</small></span><ArrowRight size={18} /></Link><Link to="/settings/imports"><ArrowDownToLine size={20} /><span><strong>설정 가져오기</strong><small>Docker · Caddy · Nginx</small></span><ArrowRight size={18} /></Link></nav>
    <SettingsForm initial={query.data} />
  </div>
}

function SettingsForm({ initial }: { initial: Settings }) {
  const [draft, setDraft] = useState(initial)
  const [saved, setSaved] = useState(false)
  const qc = useQueryClient()
  const save = useMutation({ mutationFn: () => {
    const { revision, ...body } = draft
    return api.updateSettings(body, revision)
  }, onSuccess: result => { qc.setQueryData(['settings'], result); setDraft(result); setSaved(true) } })
  const reload = useMutation({ mutationFn: api.settings, onSuccess: result => { qc.setQueryData(['settings'], result); setDraft(result); setSaved(false); save.reset() } })
  const dirty = JSON.stringify(draft) !== JSON.stringify(initial)
  function update<K extends keyof SettingsPatch>(key: K, value: SettingsPatch[K]) { setSaved(false); setDraft(current => ({ ...current, [key]: value })) }
  function submit(event: FormEvent) { event.preventDefault(); save.mutate() }
  return <form className={styles.form} onSubmit={submit}>
    <section className={styles.section}><div><h2>홈 화면</h2><p>매일 가장 먼저 보는 제목과 설명</p></div><div className="form-stack"><label className="field">홈 제목<input required maxLength={80} value={draft.title} disabled={save.isPending} onChange={event => update('title', event.target.value)} /></label><label className="field">홈 설명<input maxLength={200} value={draft.subtitle} disabled={save.isPending} onChange={event => update('subtitle', event.target.value)} /></label></div></section>
    <section className={styles.section}>
      <div><h2>화면 스타일</h2><p>밝기, 강조색, 카드 밀도</p></div>
      <div className="form-stack"><div className="form-row">
        <div className="field"><label htmlFor="appearance-theme">테마</label><select id="appearance-theme" value={draft.theme} disabled={save.isPending} onChange={event => update('theme', event.target.value as Settings['theme'])}><option value="light">밝게</option><option value="dark">어둡게</option><option value="system">시스템 설정 따르기</option></select></div>
        <div className="field"><label htmlFor="appearance-accent">강조색</label><select id="appearance-accent" value={draft.accent} disabled={save.isPending} onChange={event => update('accent', event.target.value as Settings['accent'])}><option value="blue">블루</option><option value="green">그린</option><option value="amber">앰버</option><option value="slate">슬레이트</option></select></div>
      </div><label className="checkbox-field"><input type="checkbox" checked={draft.compact} disabled={save.isPending} onChange={event => update('compact', event.target.checked)} />홈 카드를 촘촘하게 표시</label></div>
    </section>
    <section className={styles.section}><div><h2>사용자 CSS</h2><p>홈 화면의 스타일을 직접 조정</p></div><div className="form-stack"><div className="field"><label htmlFor="appearance-css">사용자 CSS</label><textarea id="appearance-css" className={styles.code} rows={12} spellCheck={false} value={draft.customCss} disabled={save.isPending} onChange={event => update('customCss', event.target.value)} placeholder={'.service-card {\n  border-radius: 16px;\n}\n\n.group-title {\n  color: var(--color-accent);\n}'} aria-describedby="css-help" /></div><p id="css-help" className="helper">저장한 CSS는 홈에만 적용됩니다. .home-dashboard, .service-card, .service-title, .service-description, .group-title을 사용할 수 있습니다. 최대 32KiB.</p><Button onClick={() => update('customCss', '')} disabled={save.isPending || !draft.customCss}>CSS 지우기</Button></div></section>
    {(save.isError || reload.isError) && <ErrorNotice error={reload.error ?? save.error} onRetry={save.error instanceof APIError && save.error.status === 409 ? () => reload.mutate() : undefined} />}
    <div className={styles.actions}><p className="helper" role="status">{dirty ? '저장하지 않은 변경사항이 있습니다.' : saved ? '설정을 저장했습니다.' : '설정은 이 서버에 저장됩니다.'}</p><Button type="submit" variant="primary" loading={save.isPending} disabled={!dirty}><Save size={17} />변경 저장</Button></div>
  </form>
}
