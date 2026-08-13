import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd())

  return {
    base: env.VITE_ROUTER_BASE || '/',
    plugins: [vue()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '~': path.resolve(__dirname),
      },
    },
    define: {
      'process.env': env,
    },
    server: {
      port: 5173,
      strictPort: false,
      open: true,
      cors: true,
      proxy: {
        '/admin': {
          target: env.VITE_API_BASE_URL || 'http://localhost:8888',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/admin/, '/admin'),
        },
      },
    },
    build: {
      target: 'ES2022',
      outDir: 'dist',
      assetsDir: 'assets',
      sourcemap: mode === 'development',
      minify: 'terser',
      terserOptions: {
        compress: {
          drop_console: mode === 'production',
        },
      },
      rollupOptions: {
        output: {
          manualChunks: {
            'element-plus': ['element-plus'],
            'vue-family': ['vue', 'vue-router', 'pinia', 'vue-i18n'],
          },
        },
      },
    },
    optimizeDeps: {
      include: ['vue', 'vue-router', 'pinia', 'element-plus', 'axios'],
    },
  }
})
