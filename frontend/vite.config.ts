import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const YOUR_HOST_ADDRESS = 'http://localhost:8080'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api/submit': {
        target: `${YOUR_HOST_ADDRESS}/webui/exploit`,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/submit/, '')
      },
      '/api/list': {
        target: `${YOUR_HOST_ADDRESS}/api/exploit_show_output`,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/list/, '')
      }
    }
  },
  build:{
    outDir: 'dist',
    sourcemap: false,
    chunkSizeWarningLimit: 1500,
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            return id.toString().split('node_modules/')[1].split('/')[0].toString();
          }
        },
        chunkFileNames: (chunkInfo) => {
          const facadeModuleId = chunkInfo.facadeModuleId ? chunkInfo.facadeModuleId.split('/') : [];
          const fileName = facadeModuleId[facadeModuleId.length - 2] || '[name]';
          return `js/${fileName}/[name].[hash].js`;
        }
      }
    }
  }
})
