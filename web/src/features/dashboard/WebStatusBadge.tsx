import type { WebStatus } from '../../api/types'
import styles from './DashboardPage.module.css'

export function WebStatusBadge({ status, enabled }: { status?: WebStatus; enabled: boolean }) {
  const state = enabled ? status?.state ?? 'checking' : 'disabled'
  const labels: Record<string, string> = { unknown: '확인 대기', checking: '확인 중', disabled: '상태 확인 꺼짐', online: '정상', auth: '인증 필요', error: 'HTTP 오류', offline: '연결 실패', timeout: '시간 초과' }
  const label = state === 'error' && status?.httpStatus === 0 ? status.message : labels[state]
  const checked = status?.checkedAt ? new Date(status.checkedAt).toLocaleString('ko-KR') : '아직 확인하지 않음'
  const detail = `Dashboard 서버에서 확인 · ${checked}${status?.httpStatus ? ` · HTTP ${status.httpStatus}` : ''}${status?.checkedAt ? ` · ${status.latencyMs}ms` : ''}${status?.checking ? ' · 다시 확인 중' : ''}`
  return <span className={`web-status ${styles.webStatus}`} data-state={state} title={detail} aria-label={`${label} · ${detail}`}><span className={styles.statusDot} aria-hidden="true" />{label}{status?.httpStatus && state !== 'online' ? <span className={styles.statusCode}>{status.httpStatus}</span> : null}</span>
}
