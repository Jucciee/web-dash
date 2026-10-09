import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Box, Cpu, HardDrive, MemoryStick, RefreshCw, Server } from 'lucide-react'
import { api } from '../../api/client'
import type { ResourceUsage, WidgetSettings } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import styles from './Monitoring.module.css'

export function bytes(value: number | null | undefined) {
  if (value == null) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let index = 0
  while (value >= 1024 && index < units.length - 1) { value /= 1024; index++ }
  return `${value.toLocaleString('ko-KR', { maximumFractionDigits: index > 1 ? 1 : 0 })} ${units[index]}`
}

function percent(value: number | null | undefined) { return value == null ? '—' : `${value.toLocaleString('ko-KR', { maximumFractionDigits: 1 })}%` }
function uptime(seconds: number | null) {
  if (seconds == null) return '—'
  const days = Math.floor(seconds / 86400), hours = Math.floor(seconds % 86400 / 3600), minutes = Math.floor(seconds % 3600 / 60)
  return days ? `${days}일 ${hours}시간` : `${hours}시간 ${minutes}분`
}

export function SampleTime({ at, interval, failed }: { at?: string; interval: number; failed: boolean }) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 10_000); return () => window.clearInterval(timer) }, [])
  if (!at) return null
  const stale = failed || now - new Date(at).getTime() > Math.max(90, interval * 3) * 1000
  return <p className={styles.sampleTime}><time dateTime={at}>{new Date(at).toLocaleTimeString('ko-KR', { hour12: false })}</time> 측정 · {stale ? <strong>오래된 측정값</strong> : interval ? `${interval}초마다 갱신` : '수동 조회'}</p>
}

function Metric({ label, value, detail }: { label: string; value: number | null | undefined; detail?: string }) {
  return <div className={styles.metric}><span>{label}</span><strong>{percent(value)}</strong>{value != null && <meter min={0} max={100} value={Math.min(100, Math.max(0, value))} aria-label={`${label} 사용량`} />}{detail && <small>{detail}</small>}</div>
}
function resourceDetail(resource: ResourceUsage | null | undefined) { return resource ? `${bytes(resource.used)} / ${bytes(resource.total)}` : '측정값 없음' }
function LoadingMetrics() { return <div className={styles.loading} role="status" aria-label="상태를 불러오는 중"><span /><span /><span /></div> }

export function HostPanel({ interval = 30, detailed = false }: { interval?: number; detailed?: boolean }) {
  const query = useQuery({ queryKey: ['monitoring', 'host'], queryFn: api.hostStatus, staleTime: 5000, refetchInterval: interval ? interval * 1000 : false, refetchIntervalInBackground: false, refetchOnWindowFocus: interval > 0, refetchOnReconnect: interval > 0 })
  const snapshot = query.data
  const scope = snapshot?.scope === 'host' ? '마운트한 Linux 호스트' : snapshot?.scope === 'runtime' ? '컨테이너 실행 환경' : 'web-dash 실행 호스트'
  return <section className={`host-widget ${styles.panel}`} aria-label="호스트 상태">
    <div className={styles.panelHead}><h2><Server size={18} aria-hidden="true" />호스트</h2><div>{!detailed && <Link to="/hosts" className="button button-ghost">자세히</Link>}<Button variant="ghost" className="icon-button" aria-label="호스트 상태 새로고침" loading={query.isFetching} disabled={query.isFetching} onClick={() => void query.refetch()}><RefreshCw size={16} /></Button></div></div>
    {query.isError && <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />}
    {query.isPending && <LoadingMetrics />}
    {snapshot && <><p className={styles.environment}>{snapshot.name || '호스트'} · {snapshot.os} · {scope}</p>
      {!snapshot.available && <p className={styles.empty}>호스트 지표를 읽지 못했습니다. 수집 경로와 권한을 확인해주세요.</p>}
      <div className={styles.metrics}><Metric label="CPU" value={snapshot.cpuPercent} detail={`${snapshot.cpuCores} 논리 코어`} /><Metric label="메모리" value={snapshot.memory?.percent} detail={resourceDetail(snapshot.memory)} /><Metric label="디스크" value={snapshot.disk?.percent} detail={resourceDetail(snapshot.disk)} /></div>
      <div className={styles.hostDetails}><span>가동 시간 <strong>{uptime(snapshot.uptimeSeconds)}</strong></span>{detailed && <span>디스크 경로 <code>{snapshot.disk?.path ?? '—'}</code></span>}</div>
      {snapshot.warnings.map(warning => <p key={warning} className={styles.warning}>{warning}</p>)}
      {detailed && snapshot.scope === 'runtime' && <p className="helper">CPU·메모리는 컨테이너에서 조회한 커널 지표이며 디스크는 선택 경로 기준입니다. 실제 Linux 호스트는 배포 문서의 호스트 마운트를 연결한 뒤 확인할 수 있습니다.</p>}
    </>}
    <SampleTime at={snapshot?.measuredAt} interval={interval} failed={query.isError} />
  </section>
}

const stateLabels: Record<string, string> = { running: '실행 중', exited: '중지됨', created: '생성됨', paused: '일시 정지', restarting: '재시작 중', removing: '삭제 중', dead: '종료됨' }
const healthLabels: Record<string, string> = { healthy: '정상', unhealthy: '비정상', starting: '확인 중', none: '검사 없음' }

export function DockerPanel({ interval = 30, detailed = false }: { interval?: number; detailed?: boolean }) {
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState('all')
  const query = useQuery({ queryKey: ['monitoring', 'docker'], queryFn: api.dockerStatus, staleTime: 5000, refetchInterval: interval ? interval * 1000 : false, refetchIntervalInBackground: false, refetchOnWindowFocus: interval > 0, refetchOnReconnect: interval > 0 })
  const snapshot = query.data
  const rows = snapshot?.containers.filter(item => [item.name, item.project, item.image].some(value => value.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase())) && (filter === 'all' || filter === 'running' && item.state === 'running' || filter === 'stopped' && item.state !== 'running' || filter === 'unhealthy' && item.health === 'unhealthy')) ?? []
  const display = detailed ? rows : [...rows].sort((a, b) => Number(b.state === 'running') - Number(a.state === 'running')).slice(0, 4)
  return <section className={`docker-widget ${styles.panel}`} aria-label="Docker 상태">
    <div className={styles.panelHead}><h2><Box size={18} aria-hidden="true" />Docker</h2><div>{!detailed && <Link to="/connections" className="button button-ghost">자세히</Link>}<Button variant="ghost" className="icon-button" aria-label="Docker 상태 새로고침" loading={query.isFetching} disabled={query.isFetching} onClick={() => void query.refetch()}><RefreshCw size={16} /></Button></div></div>
    {query.isError && <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />}
    {query.isPending && <LoadingMetrics />}
    {snapshot && (snapshot.available ? <>
      <p className={styles.environment}>{snapshot.name || 'Docker Engine'}{snapshot.os && ` · ${snapshot.os}`}{detailed && snapshot.version && ` · v${snapshot.version}`}</p>
      <div className={styles.dockerSummary}><span><strong>{snapshot.running}</strong> 실행 중</span><span><strong>{snapshot.stopped}</strong> 중지·대기</span>{detailed && <span>{snapshot.cpuCores || '—'} 코어 · {snapshot.memoryTotal ? bytes(snapshot.memoryTotal) : '메모리 정보 없음'}</span>}</div>
      {detailed && <div className={styles.filters}><label className="field">컨테이너 검색<input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="이름, 프로젝트, 이미지" /></label><label className="field">상태 필터<select value={filter} onChange={event => setFilter(event.target.value)}><option value="all">전체</option><option value="running">실행 중</option><option value="stopped">중지·대기</option><option value="unhealthy">건강 상태 비정상</option></select></label></div>}
      {snapshot.containers.length === 0 ? <p className={styles.empty}>등록된 컨테이너가 없습니다.</p> : rows.length === 0 ? <p className={styles.empty}>조건에 맞는 컨테이너가 없습니다.</p> : <div className={styles.containers}>{display.map(item => <article className={styles.container} key={item.id}><div className={styles.containerName}><strong>{item.name}</strong>{detailed && <small>{item.project || item.image}</small>}<span className={styles.state} data-state={item.state} data-health={item.health}>{stateLabels[item.state] ?? item.state} · {healthLabels[item.health] ?? item.health}</span></div><div className={styles.containerUsage}><span><Cpu size={14} aria-hidden="true" />{percent(item.cpuPercent)}</span><span><MemoryStick size={14} aria-hidden="true" />{bytes(item.memory?.used)}</span>{detailed && <small>메모리 한도 {item.memory?.total ? bytes(item.memory.total) : '—'} · 수신 {bytes(item.networkRx)} · 송신 {bytes(item.networkTx)}</small>}{item.statsError && <small className={styles.warning}>{item.statsError}</small>}</div>{detailed && <small className={styles.containerMeta}>{item.image}<br />{item.status}</small>}</article>)}</div>}
      {!detailed && rows.length > 4 && <Link className={styles.more} to="/connections">전체 {rows.length}개 컨테이너 보기</Link>}
    </> : <div className={styles.empty}><HardDrive size={24} aria-hidden="true" /><p>{snapshot.message}</p>{!detailed && <Link className="button" to="/connections">연결 안내</Link>}</div>)}
    {snapshot?.warnings.map(warning => <p key={warning} className={styles.warning}>{warning}</p>)}
    <SampleTime at={snapshot?.measuredAt} interval={interval} failed={query.isError} />
  </section>
}

export function HomeWidgets({ settings }: { settings: WidgetSettings }) {
  if (!settings.showHost && !settings.showDocker) return null
  return <div className={`home-widgets ${styles.widgets}`}>{settings.order.map(kind => kind === 'host' ? settings.showHost && <HostPanel key={kind} interval={settings.refreshSeconds} /> : settings.showDocker && <DockerPanel key={kind} interval={settings.refreshSeconds} />)}</div>
}
