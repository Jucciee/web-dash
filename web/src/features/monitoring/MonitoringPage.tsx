import { useSettings } from '../settings/useSettings'
import { DockerPanel, HostPanel } from './MonitoringPanels'
import styles from './Monitoring.module.css'

export function MonitoringPage({ kind }: { kind: 'host' | 'docker' }) {
  const settings = useSettings()
  return <div className={styles.page}><header className={styles.heading}><p className="breadcrumb">web-dash / 상태</p><h1>{kind === 'docker' ? 'Docker 상태' : '호스트 상태'}</h1></header>
    {kind === 'docker' ? <DockerPanel detailed interval={settings.data?.widgets.refreshSeconds ?? 30} /> : <HostPanel detailed interval={settings.data?.widgets.refreshSeconds ?? 30} />}
    <details className={styles.guide}><summary>{kind === 'docker' ? 'Docker 연결 설정' : '측정 환경과 디스크 설정'}</summary>{kind === 'docker' ? <><p>직접 실행하면 Linux Docker socket 또는 Windows Docker Engine에 자동 연결합니다. 다른 Engine이나 socket proxy를 사용할 때는 서버의 <code>DASHBOARD_DOCKER_HOST</code>를 지정하세요.</p><pre>DASHBOARD_DOCKER_HOST=tcp://docker-proxy:2375</pre><p>컨테이너 배포는 README의 <code>compose.monitoring.yaml</code> 예시를 사용하세요. 연결 후 새로고침하면 전체 컨테이너와 사용량이 표시됩니다.</p></> : <><p>직접 실행하면 web-dash가 설치된 시스템을 수집합니다. Docker 안에서 실행할 때는 Linux 호스트의 proc·sys·root를 읽기 전용으로 마운트하고 <code>DASHBOARD_HOST_PROC</code>, <code>DASHBOARD_HOST_SYS</code>, <code>DASHBOARD_HOST_ROOT</code>를 지정하세요.</p><p>다른 디스크는 <code>DASHBOARD_HOST_DISK</code>에 수집할 경로를 지정합니다. 표시 이름은 <code>DASHBOARD_HOST_NAME</code>으로 변경할 수 있습니다.</p></>}</details>
  </div>
}
