import { Link } from 'react-router-dom'
import { ArrowDownToLine, Box, Database, Server } from 'lucide-react'
import { useAuth } from '../../app/App'

const features = {
  imports: { title: '설정 가져오기', icon: ArrowDownToLine, description: 'Nginx 설정과 Caddyfile에서 링크 후보를 찾고, 확인한 항목을 카드로 등록합니다.', steps: ['파일 업로드 · 설정 붙여넣기', '발견한 링크 주소 확인과 수정', '선택한 링크 등록 · 재가져오기 비교'] },
  connections: { title: 'Docker 연결', icon: Box, description: '컨테이너의 실행 상태와 사용량을 조회하는 기능입니다.', steps: ['조회 전용 Docker 연결 검사', '컨테이너 목록 · 실행 상태 · 건강 상태', '링크 카드 연결 · CPU와 메모리 사용량'] },
  hosts: { title: '호스트 상태', icon: Server, description: '서버에 설치한 수집기에서 실제 호스트의 지표를 받아 표시합니다.', steps: ['수집기 등록과 인증 토큰 발급', 'CPU · RAM · 디스크 사용량', '측정 환경 · 마지막 수신 시각 · 오래된 정보 표시'] },
  settings: { title: '설정', icon: Database, description: '현재 구성은 이 서버의 SQLite에 저장됩니다. 표시 설정과 백업 기능은 다음 단계에서 추가됩니다.', steps: ['테마 · 표시 설정', '비밀정보를 제외한 구성 내보내기', '복원 전 변경 미리보기 · 일괄 적용'] },
}

export function RoadmapPage({ feature }: { feature: keyof typeof features }) {
  const item = features[feature]
  const auth = useAuth()
  return <section className="roadmap"><p className="breadcrumb">나의 홈서버 / {item.title}</p><h1>{item.title}</h1><div className="roadmap-status"><item.icon size={30} aria-hidden="true" /><span className="status-badge">다음 단계</span></div><h2>{feature === 'settings' ? '구성을 안전하게 보관하는 공간' : '기능을 연결할 준비가 되어 있습니다'}</h2><p>{item.description}</p>{feature === 'settings' && <p>현재 관리자: <strong>{auth.data?.user.username}</strong></p>}<ul>{item.steps.map(step => <li key={step}>{step}</li>)}</ul><p className="helper">현재 버전에서는 관리자 로그인, 카드·그룹 편집, 순서 변경을 사용할 수 있습니다.</p><Link className="button button-secondary" to="/">대시보드로 돌아가기</Link></section>
}
