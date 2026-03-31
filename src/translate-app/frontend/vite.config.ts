import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// Wails embed/serve: absolute "/assets/..." fails to load → white window. Relative paths required.
export default defineConfig({
  base: './',
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: '../backend/dist',
    emptyOutDir: true,
  },
})
