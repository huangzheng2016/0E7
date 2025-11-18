import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
  ],
  base: '/',
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
        secure: false,
        ws: true // 支持WebSocket代理
      },
      '/api': {
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
        entryFileNames: 'static/[name].[hash].js',
        chunkFileNames: 'static/[name].[hash].js',
        assetFileNames: (assetInfo) => {
          let fileName = assetInfo.name || '[name]';
          if(fileName[0]==='.') fileName = fileName.slice(1);
          return `static/${fileName}.[hash][extname]`;
        },
        manualChunks(id) {
          // 只对 node_modules 中的依赖进行拆分
          if (id.includes('node_modules')) {
            // 将大型库拆分出来
            if (id.includes('element-plus')) {
              return 'element-plus';
            }
            if (id.includes('vuex')) {
              return 'vuex';
            }
            // CodeMirror 相关包合并到 vendor，避免初始化顺序问题
            // 其他依赖合并到 vendor
            return 'vendor';
          }
        }
      }
    }
  }
})
