import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    // Forward API calls to Go. The browser sees one origin, so there's no CORS
    // and the auth cookies stay first-party.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})