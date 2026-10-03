import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource/space-grotesk/latin-500.css'
import '@fontsource/space-grotesk/latin-600.css'
import '@fontsource-variable/noto-sans-kr'
import { App } from './app/App'
import './styles/global.css'

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)
