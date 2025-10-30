import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
  ],
  base: '/static/',
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    proxy: {
      '/webui': {
        target: 'http://localhost:6102',
        changeOrigin: true,
        secure: false
      }
    }
  },
  build:{
    outDir: '../dist',
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
          let fileName = chunkInfo.name || '[name]';
          if(fileName[0]==='.') fileName = fileName.slice(1);
          console.log('fl:'+fileName);
          return `js/${fileName}.[hash].js`;
        },
        assetFileNames: (assetInfo) => {
          let fileName = assetInfo.name || '[name]';
          if(fileName[0]==='.') fileName = fileName.slice(1);
          return `css/${fileName}.[hash][extname]`;
        }
      }
    }
  }
})
