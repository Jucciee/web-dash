import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { RefreshCw } from 'lucide-react'
import { api } from '../../api/client'
import type { Addon, AddonManifest, AddonSnapshot } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import { ServiceIcon } from '../dashboard/ServiceIcon'
import { SampleTime } from '../monitoring/MonitoringPanels'
import styles from './Addons.module.css'

export function AddonValues({ manifest, snapshot }: { manifest: AddonManifest; snapshot?: AddonSnapshot }) {
  return <div className={styles.metrics} data-display={manifest.display}>{manifest.fields.map((field, index) => {
    const value = snapshot?.values[index]
    return <div className={`addon-metric ${styles.metric}`} key={`${field.path}-${index}`}><span>{field.label}</span><strong>{value && !value.missing ? value.value : '—'}{value && !value.missing && value.unit && <small>{value.unit}</small>}</strong>{value?.missing && <small>경로·표시 형식 확인</small>}</div>
  })}</div>
}
export function AddonWidget({ addon }: { addon: Addon }) {
  const interval = addon.manifest.refreshSeconds
  const query = useQuery({ queryKey: ['addons', 'snapshot', addon.id, addon.revision], queryFn: () => api.readAddon(addon.id), staleTime: 5000, refetchInterval: interval ? interval * 1000 : false, refetchIntervalInBackground: false, refetchOnWindowFocus: interval > 0, refetchOnReconnect: interval > 0 })
  return <section className={`addon-widget ${styles.widget}`} data-addon={addon.id} aria-label={`${addon.manifest.name} 애드온`}><div className={styles.widgetHead}><h2><ServiceIcon iconKey={addon.manifest.icon} size={18} />{addon.manifest.name}</h2><Button variant="ghost" className="icon-button" aria-label={`${addon.manifest.name} 새로고침`} loading={query.isFetching} onClick={() => void query.refetch()}><RefreshCw size={16} /></Button></div>
    {query.isPending && <p className="helper" role="status">불러오는 중…</p>}{query.isError && <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />}
    {query.data?.available ? <AddonValues manifest={addon.manifest} snapshot={query.data} /> : query.data && <div className={styles.failed}><p>{query.data.message}</p><Link to="/settings/addons">애드온 설정</Link></div>}
    <SampleTime at={query.data?.measuredAt} interval={interval} failed={query.isError} />
  </section>
}
export function AddonWidgets() {
  const query = useQuery({ queryKey: ['addons', 'list'], queryFn: api.addons })
  if (query.isError) return <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
  const enabled = query.data?.filter(addon => addon.enabled) ?? []
  if (!enabled.length) return null
  return <div className={`addon-widgets ${styles.widgets}`}>{enabled.map(addon => <AddonWidget key={addon.id} addon={addon} />)}</div>
}

export function AddonPreviewWidgets() {
  const query = useQuery({ queryKey: ['addons', 'list'], queryFn: api.addons })
  const enabled = query.data?.filter(addon => addon.enabled).slice(0, 3) ?? []
  if (!enabled.length) return null
  return <div className={`addon-widgets ${styles.widgets}`}>{enabled.map(addon => <section className={`addon-widget ${styles.widget}`} key={addon.id}><div className={styles.widgetHead}><h2><ServiceIcon iconKey={addon.manifest.icon} size={18} />{addon.manifest.name}</h2></div><AddonValues manifest={addon.manifest} /></section>)}</div>
}
