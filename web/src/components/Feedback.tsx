import { AlertCircle, ServerOff } from 'lucide-react'
import { Button } from './Button'
import { errorMessage } from '../api/client'

export function ErrorNotice({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  return <div className="error-notice" role="alert"><AlertCircle size={18} aria-hidden="true" /><div><p>{errorMessage(error)}</p>{onRetry && <Button variant="ghost" onClick={onRetry}>다시 시도</Button>}</div></div>
}

export function LoadingPage() {
  return <main className="loading-page" role="status"><span className="wordmark">Dashboard<span className="brand-dot" /></span><p>대시보드를 불러오는 중…</p></main>
}

export function ConnectionError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  return <main className="loading-page"><ServerOff size={28} aria-hidden="true" /><h1>서버에 연결할 수 없습니다</h1><ErrorNotice error={error} onRetry={onRetry} /></main>
}
