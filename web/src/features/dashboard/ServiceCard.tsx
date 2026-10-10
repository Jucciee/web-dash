import { ArrowDown, ArrowUp, ArrowUpRight, EyeOff, Pencil, Trash2 } from 'lucide-react'
import type { AppearanceSettings, Service, WebStatus } from '../../api/types'
import { defaultAppearance } from '../settings/defaults'
import { Button } from '../../components/Button'
import { ServiceIcon } from './ServiceIcon'
import { WebStatusBadge } from './WebStatusBadge'
import styles from './DashboardPage.module.css'

export function ServiceCard({ service, status, onEdit, onDelete, onMove, first, last, busy, manage = true, fit = false, appearance = defaultAppearance }: { service: Service; status?: WebStatus; onEdit: () => void; onDelete: () => void; onMove: (direction: -1 | 1) => void; first: boolean; last: boolean; busy: boolean; manage?: boolean; fit?: boolean; appearance?: AppearanceSettings }) {
  let address = service.url
  try { const url = new URL(service.url); address = url.host + (url.pathname === '/' ? '' : url.pathname) + url.search + url.hash } catch { /* Existing data is still visible for editing. */ }
  if (!manage) return <a className={`service-card ${styles.homeCard} ${fit ? styles.homeCardFit : ''}`} href={service.url} target="_blank" rel="noopener noreferrer" title={service.name}>{appearance.showIcons && <span className={styles.serviceIcon}><ServiceIcon service={service} iconKey={service.iconKey} /></span>}<span className={styles.homeCardCopy}><strong className="service-title">{service.name}</strong><span className={styles.homeCardDetails}>{appearance.showDescriptions && service.description && <small className="service-description" title={service.description}>{service.description}</small>}{appearance.showUrls && <small className="service-url">{address}</small>}{appearance.showWebStatus && <WebStatusBadge status={status} enabled={service.webCheckEnabled} />}</span></span><ArrowUpRight size={16} aria-hidden="true" /></a>
  return <article className={`${styles.card} ${service.hidden ? styles.hiddenCard : ''}`}>
    <div className={styles.cardTop}><span className={styles.serviceIcon}><ServiceIcon service={service} iconKey={service.iconKey} /></span><Button className="icon-button" variant="ghost" aria-label={`${service.name} 편집`} onClick={onEdit} disabled={busy}><Pencil size={16} /></Button></div>
    <a className={styles.serviceLink} href={service.url} target="_blank" rel="noopener noreferrer"><span>{service.name}</span><ArrowUpRight size={18} aria-hidden="true" /><small className="mono">{address}</small></a>
    {service.description && <p className={styles.description}>{service.description}</p>}
    <div className={styles.statusLine}><WebStatusBadge status={status} enabled={service.webCheckEnabled} />{service.hidden && <span><EyeOff size={14} aria-hidden="true" />숨긴 링크</span>}</div>
    <div className={styles.cardTools}><span>순서</span><Button variant="ghost" className="icon-button" aria-label={`${service.name} 위로 이동`} disabled={first || busy} onClick={() => onMove(-1)}><ArrowUp size={16} /></Button><Button variant="ghost" className="icon-button" aria-label={`${service.name} 아래로 이동`} disabled={last || busy} onClick={() => onMove(1)}><ArrowDown size={16} /></Button><Button variant="ghost" className={`icon-button ${styles.deleteButton}`} aria-label={`${service.name} 삭제`} disabled={busy} onClick={onDelete}><Trash2 size={16} /></Button></div>
  </article>
}
