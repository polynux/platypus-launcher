import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  // Under the desktop shell (apps/desktop/scripts/run.mjs dev) the page is
  // app://platypus and Vite sits behind it, so hot reload must dial Vite itself.
  server: process.env.PLATYPUS_VITE_PORT
    ? { hmr: { protocol: 'ws', host: '127.0.0.1', clientPort: Number(process.env.PLATYPUS_VITE_PORT) } }
    : undefined,
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
})
