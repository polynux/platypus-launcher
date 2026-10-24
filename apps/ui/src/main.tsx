import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import './lib/i18n'
import { applyCachedTheme } from './lib/theme'
import App from './App'

// The theme from last time, before the first paint: no flash of the default.
applyCachedTheme()

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
