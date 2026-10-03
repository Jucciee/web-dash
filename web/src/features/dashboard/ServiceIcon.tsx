import { Code2, Database, Files, Globe, House, Images, Network, Play, Server } from 'lucide-react'
import { useState } from 'react'
import type { Service } from '../../api/types'

const icons = { server: Server, photos: Images, media: Play, files: Files, code: Code2, home: House, network: Network, database: Database }

export const iconOptions = [
  { key: 'server', label: '서버' }, { key: 'photos', label: '사진' }, { key: 'media', label: '미디어' },
  { key: 'files', label: '파일' }, { key: 'code', label: '개발' }, { key: 'home', label: '홈' },
  { key: 'network', label: '네트워크' }, { key: 'database', label: '데이터베이스' },
] as const

export function ServiceIcon({ iconKey, size = 22, service }: { iconKey: string; size?: number; service?: Service }) {
  const Icon = icons[iconKey as keyof typeof icons] ?? Globe
  if (service && service.iconMode !== 'builtin') return <ImageIcon key={`${service.id}-${service.revision}`} src={`/api/v1/services/${service.id}/icon?revision=${service.revision}`} size={size} fallback={<Icon size={size} aria-hidden="true" />} />
  return <Icon size={size} aria-hidden="true" />
}

function ImageIcon({ src, size, fallback }: { src: string; size: number; fallback: React.ReactNode }) {
  const [failed, setFailed] = useState(false)
  return failed ? fallback : <img className="link-icon-image" src={src} alt="" width={size} height={size} onError={() => setFailed(true)} loading="lazy" />
}
