import type { AppearanceSettings, Settings, WidgetSettings } from '../../api/types'

export const defaultAppearance: AppearanceSettings = {
  layout: 'grid', columns: 0, width: 'normal', cardStyle: 'card', iconSize: 'medium', radius: 'soft', fontSize: 'normal', fontFamily: 'noto',
  background: 'plain', backgroundImage: '', backgroundDim: 60, accentColor: '',
  showTitle: true, showClock: true, showSearch: true, showIcons: true, showDescriptions: true, showUrls: false, showWebStatus: true, showGroupCounts: true, showFooter: true, collapsibleGroups: false,
}
export const defaultWidgets: WidgetSettings = { showHost: true, showDocker: true, refreshSeconds: 30, order: ['host', 'docker'] }

export const presets: { name: string; description: string; settings: Partial<Settings> }[] = [
  { name: '기본', description: '밝은 화면 · 자동 배치', settings: { theme: 'light', accent: 'blue', compact: false, appearance: defaultAppearance, widgets: defaultWidgets } },
  { name: '홈 랩', description: '어두운 화면 · 상태 중심', settings: { theme: 'dark', accent: 'green', compact: false, appearance: { ...defaultAppearance, width: 'wide', background: 'grid', showUrls: true }, widgets: defaultWidgets } },
  { name: '미니멀', description: '링크에 집중', settings: { theme: 'system', accent: 'slate', compact: false, appearance: { ...defaultAppearance, width: 'narrow', cardStyle: 'list', showClock: false, showDescriptions: false, showWebStatus: false, showGroupCounts: false }, widgets: { ...defaultWidgets, showHost: false, showDocker: false } } },
  { name: '컴팩트', description: '작은 카드 · 넓은 화면', settings: { theme: 'dark', accent: 'violet', compact: true, appearance: { ...defaultAppearance, width: 'wide', cardStyle: 'tile', iconSize: 'small', showDescriptions: false, fontSize: 'small' }, widgets: defaultWidgets } },
]
