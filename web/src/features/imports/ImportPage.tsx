import { useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowDownToLine, Check, FileText, FolderSearch, RefreshCw, Upload, X } from 'lucide-react'
import { api } from '../../api/client'
import type { ImportFile, ImportPreview, ImportSelection } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import styles from './ImportPage.module.css'

type Kind = 'docker' | 'nginx' | 'caddy'
type Row = ImportSelection & { selected: boolean; duplicate: boolean }
const defaultEntry = (kind: Kind) => kind === 'caddy' ? 'Caddyfile' : 'nginx.conf'

export function ImportPage() {
  const qc = useQueryClient()
  const list = useQuery({ queryKey: ['dashboards', 'list'], queryFn: api.dashboards })
  const dashboardId = list.data?.[0]?.id
  const dashboard = useQuery({ queryKey: ['dashboards', dashboardId], queryFn: () => api.dashboard(dashboardId!), enabled: !!dashboardId })
  const [kind, setKind] = useState<Kind>('docker')
  const [folder, setFolder] = useState('')
  const [publicHost, setPublicHost] = useState(window.location.hostname)
  const [mode, setMode] = useState<'paste' | 'upload'>('paste')
  const [entry, setEntry] = useState('nginx.conf')
  const [text, setText] = useState('')
  const [files, setFiles] = useState<ImportFile[]>([])
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [rows, setRows] = useState<Row[]>([])
  const [validation, setValidation] = useState<string | null>(null)
  const [fileReading, setFileReading] = useState(false)
  const [created, setCreated] = useState<number | null>(null)
  const commitKey = useRef(crypto.randomUUID())
  const resultsRef = useRef<HTMLHeadingElement>(null)
  const scan = useMutation({ mutationFn: () => {
    if (kind === 'docker') return api.scanDocker({ dashboardId: dashboardId!, folder, publicHost })
    const inputFiles = mode === 'paste' ? [{ path: entry, content: text }] : files
    if (!inputFiles.length || inputFiles.some(file => !file.content.trim())) throw new Error('설정 내용을 입력하거나 파일을 선택해주세요.')
    if (inputFiles.some(file => new TextEncoder().encode(file.content).length > 2 * 1024 * 1024)) throw new Error('파일 하나는 2MiB까지 분석할 수 있습니다.')
    return api.importPreview({ dashboardId: dashboardId!, kind, sourceName: entry, entryPath: entry, files: inputFiles })
  }, onSuccess: result => {
    setPreview(result)
    setRows(result.candidates.map(candidate => ({ candidateId: candidate.id, selected: candidate.diff !== 'duplicate' && !!candidate.url, duplicate: candidate.diff === 'duplicate', groupId: dashboard.data!.groups[0]?.id ?? '', name: candidate.name, url: candidate.url, iconKey: 'server', iconMode: 'site', iconUrl: '', iconAssetId: '', webCheckEnabled: true, description: '', hidden: false })))
    commitKey.current = crypto.randomUUID()
    setCreated(null)
    requestAnimationFrame(() => resultsRef.current?.focus())
  } })
  const commit = useMutation({ mutationFn: () => api.commitImport(preview!.id, { dashboardRevision: preview!.dashboardRevision, items: rows.filter(row => row.selected).map(({ selected: _selected, duplicate: _duplicate, ...item }) => item) }, commitKey.current), onSuccess: async result => {
    setCreated(result.serviceIds.length)
    setPreview(null)
    setRows([])
    await qc.invalidateQueries({ queryKey: ['dashboards'] })
  } })
  const busy = scan.isPending || commit.isPending || fileReading
  const selectedCount = rows.filter(row => row.selected).length
  function resetPreview() { setPreview(null); setRows([]); setCreated(null); setValidation(null); scan.reset(); commit.reset() }
  function changeKind(value: Kind) { setKind(value); setEntry(defaultEntry(value)); setText(''); setFiles([]); resetPreview() }
  function updateRow(index: number, patch: Partial<Row>) { setRows(current => current.map((row, i) => i === index ? { ...row, ...patch } : row)) }
  async function readFiles(selected: FileList | null) {
    if (!selected?.length) return
    setFileReading(true)
    setValidation(null)
    try {
      const picked = Array.from(selected)
      if (picked.length > 50 || picked.some(file => file.size > 2 * 1024 * 1024) || picked.reduce((total, file) => total + file.size, 0) > 10 * 1024 * 1024) throw new Error('파일은 50개까지, 개별 2MiB·전체 10MiB까지 선택할 수 있습니다.')
      const contents = await Promise.all(picked.map(async file => ({ path: file.webkitRelativePath || file.name, content: await file.text() })))
      setFiles(contents)
      setEntry(contents.find(file => /^(Caddyfile|nginx\.conf)$/i.test(file.path))?.path ?? contents[0].path)
      setPreview(null)
      setCreated(null)
    } catch (error) { setValidation(error instanceof Error ? error.message : '파일을 읽지 못했습니다.') }
    finally { setFileReading(false) }
  }
  function submit(event: FormEvent) { event.preventDefault(); setValidation(null); setPreview(null); setCreated(null); commit.reset(); scan.mutate() }
  if (list.isError || dashboard.isError) return <ErrorNotice error={list.error ?? dashboard.error} />
  if (list.isPending || (dashboardId && dashboard.isPending)) return <p role="status" className="helper">대시보드를 불러오는 중…</p>
  if (!dashboard.data) return <ErrorNotice error={new Error('대시보드를 찾을 수 없습니다.')} />
  return <div className={styles.page}>
    <header className={styles.heading}><p className="breadcrumb">설정 / 가져오기</p><h1>기존 링크 가져오기</h1><p>설정에서 접속 주소를 찾고, 필요한 링크만 골라 등록하세요.</p></header>
    <form className={styles.sourceForm} onSubmit={submit}>
      <div className={styles.kindChoices} role="group" aria-label="설정 종류">{([{ value: 'docker', label: 'Docker', description: '폴더 · 실행 중인 포트' }, { value: 'caddy', label: 'Caddy', description: 'Caddyfile · 사이트 주소' }, { value: 'nginx', label: 'Nginx', description: '설정 파일 · 서버 주소' }] as const).map(item => <Button key={item.value} className={kind === item.value ? styles.chosen : ''} aria-pressed={kind === item.value} onClick={() => changeKind(item.value)} disabled={busy}><span>{item.label}<small>{item.description}</small></span></Button>)}</div>
      {kind === 'docker' ? <div key="docker" className="form-stack"><label className="field">Compose 폴더 경로<input required value={folder} disabled={busy} onChange={event => setFolder(event.target.value)} placeholder="D:\\Docker 또는 /srv/compose" aria-describedby="folder-help" /></label><p id="folder-help" className="helper">Dashboard 서버에서 접근할 수 있는 폴더를 입력하세요. 하위 폴더의 compose.yml·compose.yaml·docker-compose.yml·docker-compose.yaml을 찾아 실행 중인 컨테이너와 연결합니다.</p><label className="field">접속 호스트<input required value={publicHost} disabled={busy} onChange={event => setPublicHost(event.target.value)} placeholder="192.168.1.10" /><small>전체 인터페이스에 공개된 포트의 주소에 사용합니다. 호스트 이름 또는 IP만 입력하세요.</small></label><p className={styles.sourceNote}>Docker가 실제로 공개한 TCP 포트를 가져옵니다. 설정 파일의 포트만으로 실행 상태를 판단하지 않습니다.</p></div> : <div key={kind + mode} className="form-stack"><div className={styles.modeChoices}><Button aria-pressed={mode === 'paste'} onClick={() => { setMode('paste'); resetPreview(); setEntry(defaultEntry(kind)) }} disabled={busy}><FileText size={16} />붙여넣기</Button><Button aria-pressed={mode === 'upload'} onClick={() => { setMode('upload'); resetPreview(); if (files.length) setEntry(files[0].path) }} disabled={busy}><Upload size={16} />파일 업로드</Button></div>{mode === 'paste' ? <><label className="field">파일 이름<input required maxLength={240} value={entry} onChange={event => setEntry(event.target.value)} disabled={busy} /></label><div className="field"><label htmlFor="import-content">설정 내용</label><textarea id="import-content" className={styles.code} rows={12} required value={text} onChange={event => setText(event.target.value)} disabled={busy} spellCheck={false} placeholder={kind === 'caddy' ? 'photos.example.com {\n    reverse_proxy photos:3000\n}' : 'server {\n    listen 443 ssl;\n    server_name photos.example.com;\n    location / {\n        proxy_pass http://photos:3000;\n    }\n}'} /></div></> : <><label className="field">설정 파일 선택<input type="file" multiple disabled={busy} onChange={event => void readFiles(event.target.files)} /><small>참조하는 include/import 파일도 함께 선택하세요.</small></label>{files.map((file, index) => <div className={styles.fileRow} key={index}><label className="field">파일 경로 {index + 1}<input value={file.path} disabled={busy} onChange={event => { const value = event.target.value; setFiles(current => current.map((item, i) => i === index ? { ...item, path: value } : item)); if (entry === file.path) setEntry(value) }} /></label><Button variant="ghost" className="icon-button" aria-label={`${file.path} 제외`} disabled={busy} onClick={() => { setFiles(current => current.filter((_, i) => i !== index)); if (entry === file.path) setEntry(files.find((_, i) => i !== index)?.path ?? '') }}><X size={18} /></Button></div>)}{files.length > 0 && <label className="field">분석 시작 파일<select value={entry} disabled={busy} onChange={event => setEntry(event.target.value)}>{files.map((file, index) => <option key={index} value={file.path}>{file.path}</option>)}</select></label>}</>}<p className={styles.sourceNote}>파일 내용에서 주소를 읽습니다. 참조 경로는 함께 올린 파일 안에서만 찾습니다.</p></div>}
      {(validation || scan.isError) && <ErrorNotice error={validation ? new Error(validation) : scan.error} />}
      <div className={styles.sourceActions}><span className="helper">미리보기 후 선택 등록 · 30분간 유효</span><Button type="submit" variant="primary" loading={scan.isPending} disabled={busy || (kind !== 'docker' && mode === 'upload' && files.length === 0)}>{kind === 'docker' ? <FolderSearch size={17} /> : <ArrowDownToLine size={17} />}{kind === 'docker' ? '폴더 검색' : '미리보기'}</Button></div>
    </form>
    {created !== null && <section className={styles.completed} role="status"><Check size={22} /><div><h2>{created}개 링크를 가져왔습니다</h2><p>링크 관리에서 이름·아이콘·순서를 더 편집할 수 있습니다.</p></div><Link to="/" className="button">홈에서 보기</Link></section>}
    {preview && <section className={styles.preview} aria-label="가져오기 미리보기"><div className={styles.previewHeading}><div><h2 ref={resultsRef} tabIndex={-1}>가져오기 미리보기</h2><p>{preview.candidates.length}개 후보 · {selectedCount}개 선택</p></div><Button variant="ghost" onClick={resetPreview} disabled={busy}><RefreshCw size={16} />다시 선택</Button></div>{preview.warnings.length > 0 && <ul className={styles.warnings}>{preview.warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul>}{!preview.candidates.length && <p className={styles.noResults}>가져올 주소가 없습니다. 위 안내와 실행 상태를 확인해주세요.</p>}{rows.length > 0 && <label className="checkbox-field"><input type="checkbox" checked={rows.every(row => row.duplicate || row.selected)} disabled={busy} onChange={event => setRows(current => current.map(row => ({ ...row, selected: !row.duplicate && event.target.checked })))} />등록 가능한 항목 모두 선택</label>}{rows.map((row, index) => { const candidate = preview.candidates[index]; return <div className={styles.candidate} key={row.candidateId}><div className={styles.candidateHeading}><label className="checkbox-field"><input type="checkbox" checked={row.selected} disabled={busy || row.duplicate} onChange={event => updateRow(index, { selected: event.target.checked })} />{candidate.name}</label><span className="status-badge">{row.duplicate ? '이미 등록됨' : '새 링크'}</span></div><p className={styles.file}>{candidate.file}{candidate.line > 0 ? `:${candidate.line}` : ''}</p><div className={styles.candidateFields}><label className="field">링크 이름 {index + 1}<input value={row.name} maxLength={80} disabled={busy || row.duplicate} onChange={event => updateRow(index, { name: event.target.value })} /></label><label className="field">접속 주소 {index + 1}<input type="url" value={row.url} maxLength={2048} placeholder="https://service.example.com" disabled={busy || row.duplicate} onChange={event => updateRow(index, { url: event.target.value })} /></label><label className="field">그룹 {index + 1}<select value={row.groupId} disabled={busy || row.duplicate} onChange={event => updateRow(index, { groupId: event.target.value })}>{dashboard.data!.groups.map(group => <option key={group.id} value={group.id}>{group.name}</option>)}</select></label></div>{candidate.upstreams.length > 0 && <p className={styles.upstream}>연결 대상 · {candidate.upstreams.join(', ')}</p>}{candidate.warnings.length > 0 && <ul className={styles.candidateWarnings}>{candidate.warnings.map((warning, i) => <li key={i}>{warning}</li>)}</ul>}</div> })}{!dashboard.data.groups.length && <p className={styles.noResults}>등록할 그룹이 없습니다. <Link to="/settings/services">링크 관리에서 그룹을 추가하세요.</Link></p>}{commit.isError && <ErrorNotice error={commit.error} />}<div className={styles.sourceActions}><span className="helper">선택한 주소와 이름을 확인한 뒤 등록하세요.</span><Button variant="primary" loading={commit.isPending} disabled={busy || selectedCount === 0 || !dashboard.data.groups.length} onClick={() => commit.mutate()}>{selectedCount}개 링크 등록</Button></div></section>}
  </div>
}
