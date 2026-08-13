/**
 * 日志工具
 */

import { ENV } from '@/config/environment'

type LogLevel = 'debug' | 'info' | 'warn' | 'error'

function getConsoleOutput(level: LogLevel) {
  switch (level) {
    case 'debug':
      return console.debug
    case 'info':
      return console.info
    case 'warn':
      return console.warn
    case 'error':
      return console.error
    default:
      return console.info
  }
}

class Logger {
  private enableLog = ENV.ENABLE_LOG

  private log(level: LogLevel, message: string, data?: unknown) {
    if (!this.enableLog) return

    const timestamp = new Date().toLocaleTimeString()
    const prefix = `[${timestamp}] [${level.toUpperCase()}]`

    const styles = {
      debug: 'color: #0ea5e9; font-weight: bold;',
      info: 'color: #10b981; font-weight: bold;',
      warn: 'color: #f59e0b; font-weight: bold;',
      error: 'color: #ef4444; font-weight: bold;',
    }

    const output = getConsoleOutput(level)
    if (data !== undefined) {
      output(`%c${prefix} ${message}`, styles[level], data)
    } else {
      output(`%c${prefix} ${message}`, styles[level])
    }
  }

  debug(message: string, data?: unknown) {
    this.log('debug', message, data)
  }

  info(message: string, data?: unknown) {
    this.log('info', message, data)
  }

  warn(message: string, data?: unknown) {
    this.log('warn', message, data)
  }

  error(message: string, data?: unknown) {
    this.log('error', message, data)
  }
}

export const logger = new Logger()
