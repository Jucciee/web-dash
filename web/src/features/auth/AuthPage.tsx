import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, KeyRound } from 'lucide-react'
import { api } from '../../api/client'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import styles from './AuthPage.module.css'

export function AuthPage({ setup = false }: { setup?: boolean }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [setupToken, setSetupToken] = useState('')
  const [validation, setValidation] = useState<string | null>(null)
  const qc = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => setup ? api.setup({ username: username.trim(), password, setupToken: setupToken.trim() }) : api.login({ username: username.trim(), password }),
    onSuccess: result => {
      qc.setQueryData(['setup'], { needsSetup: false })
      qc.setQueryData(['auth'], result)
    },
  })
  function submit(event: FormEvent) {
    event.preventDefault()
    if (setup && (Array.from(password).length < 12 || new TextEncoder().encode(password).length > 72)) { setValidation('비밀번호는 12자 이상, UTF-8 기준 72바이트 이하로 입력해주세요.'); return }
    setValidation(null)
    mutation.mutate()
  }
  return <main className={styles.page}>
    <header className={styles.authBrand}><span className="wordmark">web-dash<span className="brand-dot" /></span></header>
    <section className={styles.formArea}>
      <div className={styles.formWrap}><KeyRound size={24} className={styles.keyIcon} aria-hidden="true" /><h1>{setup ? '관리자 설정' : '로그인'}</h1>
        <form onSubmit={submit} className="form-stack" aria-label={setup ? '관리자 설정' : '로그인'}>
          {setup && <div className="field"><label htmlFor="setup-token">초기 설정 토큰</label><input id="setup-token" name="setupToken" value={setupToken} onChange={event => setSetupToken(event.target.value)} autoComplete="off" required maxLength={256} disabled={mutation.isPending} aria-describedby="token-help" autoFocus /><small id="token-help">서버의 data/setup-token 파일에 있는 값을 입력하세요. 설치 시 토큰을 따로 지정했다면 그 값을 사용하세요.</small></div>}
          <label className="field">관리자 아이디<input name="username" value={username} onChange={event => setUsername(event.target.value)} autoComplete="username" minLength={setup ? 3 : undefined} maxLength={64} required autoFocus={!setup} disabled={mutation.isPending} placeholder="관리자 아이디" /></label>
          <div className="field"><label htmlFor="auth-password">비밀번호</label><input id="auth-password" name="password" type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete={setup ? 'new-password' : 'current-password'} minLength={setup ? 12 : undefined} required disabled={mutation.isPending} aria-describedby={setup ? 'password-help' : undefined} />{setup && <small id="password-help">12자 이상 · 최대 72 UTF-8바이트 (한글은 보통 3바이트)</small>}</div>
          {(validation || mutation.isError) && <ErrorNotice error={validation ? new Error(validation) : mutation.error} />}
          <Button type="submit" variant="primary" loading={mutation.isPending}>{mutation.isPending ? '확인하는 중…' : setup ? '관리자 계정 만들기' : '로그인'}{!mutation.isPending && <ArrowRight size={17} aria-hidden="true" />}</Button>
        </form>
      </div>
    </section>
  </main>
}
