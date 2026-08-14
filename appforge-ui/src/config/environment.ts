/**
 * 环境变量配置
 * 支持 .env 文件加载，见 vite.config.ts
 */

export const ENV = {
  // API 配置
  API_BASE_URL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8888',
  API_TIMEOUT: Number(import.meta.env.VITE_API_TIMEOUT) || 15000,

  // 应用配置
  APP_NAME: import.meta.env.VITE_APP_NAME || 'Admin UI',
	APP_MODE: import.meta.env.VITE_APP_MODE === 'agent' ? 'agent' : 'admin',
  APP_ENV: import.meta.env.MODE,
  IS_DEV: import.meta.env.DEV,
  IS_PROD: import.meta.env.PROD,

  // 路由配置
  ROUTER_BASE: import.meta.env.VITE_ROUTER_BASE || '/',

  // 功能开关
  ENABLE_MOCK: import.meta.env.VITE_ENABLE_MOCK === 'true',
  ENABLE_LOG: import.meta.env.VITE_ENABLE_LOG === 'true',
}

export const IS_AGENT = ENV.APP_MODE === 'agent'
export const AUTH_API_BASE = IS_AGENT ? '/agent' : '/admin/system'
export const PLATFORM_API_BASE = IS_AGENT ? '/agent/core' : '/admin/core'
export const TEAM_API_BASE = IS_AGENT ? '/agent/team' : '/admin/system'

export default ENV
