import { useQuery } from '@tanstack/react-query'
import { api } from '../../api/client'
import type { Dashboard, Settings, WebStatus } from '../../api/types'
import { ErrorNotice } from '../../components/Feedback'
import { HomeCanvas } from './HomeCanvas'
import { catalogFor, reconcileLayout } from './layout'

export function CustomHome({ dashboard, settings, statuses, search, preview = false }: { dashboard: Dashboard; settings: Settings; statuses?: WebStatus[]; search?: string; preview?: boolean }) {
  const addons = useQuery({ queryKey: ['addons', 'list'], queryFn: api.addons })
  const catalog = catalogFor(dashboard, addons.data ?? [], settings)
  // Waiting avoids briefly removing addon references while metadata is loading.
  if (addons.isPending) return <p className="helper" role="status">홈 항목을 불러오는 중…</p>
  return <>{addons.isError && <ErrorNotice error={addons.error} onRetry={() => void addons.refetch()} />}<HomeCanvas key={settings.revision} layout={reconcileLayout(settings.homeLayout, catalog)} catalog={catalog} settings={settings} statuses={statuses} search={search} preview={preview} /></>
}
