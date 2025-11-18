<template>
  <div class="log-viewer">
    <div ref="terminalContainer" class="terminal-container"></div>
    <!-- 搜索框 -->
    <div v-if="showSearch" class="search-box">
      <el-input
        ref="searchInputRef"
        v-model="searchTerm"
        placeholder="搜索日志..."
        @keydown.esc="closeSearch"
        @keydown.enter.prevent="findNext"
        @keydown.shift.enter.prevent="findPrevious"
        @keydown.up.prevent="findPrevious"
        @keydown.down.prevent="findNext"
        class="search-input"
        size="small"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
        <template #suffix>
          <span class="search-hint">
            <kbd>↑</kbd>/<kbd>↓</kbd> 导航
            <kbd>Enter</kbd> 下一个
            <kbd>Esc</kbd> 关闭
          </span>
        </template>
      </el-input>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { Search } from '@element-plus/icons-vue'
import '@xterm/xterm/css/xterm.css'

const terminalContainer = ref<HTMLElement | null>(null)
const searchInputRef = ref()
const showSearch = ref(false)
const searchTerm = ref('')
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let searchAddon: SearchAddon | null = null
let ws: WebSocket | null = null
let reconnectTimer: NodeJS.Timeout | null = null
const reconnectDelay = 3000 // 3秒后重连
let isUserAtBottom = true // 用户是否在底部

// 全局键盘事件处理函数
const handleKeyDown = (e: KeyboardEvent) => {
  // Ctrl+F 或 Cmd+F 打开搜索
  if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
    e.preventDefault()
    if (!showSearch.value) {
      showSearch.value = true
      nextTick(() => {
        searchInputRef.value?.focus()
      })
    }
  }
  // Esc 关闭搜索
  if (e.key === 'Escape' && showSearch.value) {
    closeSearch()
  }
}

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/webui/log/ws`
  
  try {
    ws = new WebSocket(wsUrl)
    
    ws.onopen = () => {
      console.log('日志WebSocket连接已建立')
      if (terminal) {
        terminal.writeln('\x1b[32m[已连接到日志服务器]\x1b[0m')
      }
    }
    
    ws.onmessage = (event) => {
      if (terminal) {
        // 清理接收到的消息，去除多余空格
        let message = event.data
        // 去除末尾的换行符（如果有）
        message = message.replace(/\r?\n$/, '')
        // 去除行首的所有空白字符
        message = message.replace(/^[\s\t]+/, '')
        // 将多个连续空格替换为单个空格
        message = message.replace(/ +/g, ' ')
        // 去除行尾空格
        message = message.replace(/[\s\t]+$/, '')
        // 写入终端，添加换行符
        if (message) {
          terminal.writeln(message)
          // 只有当用户在底部时才自动滚动到底部
          if (isUserAtBottom) {
            terminal.scrollToBottom()
          }
        }
      }
    }
    
    ws.onerror = (error) => {
      console.error('WebSocket错误:', error)
      if (terminal) {
        terminal.writeln('\x1b[31m[连接错误，正在重连...]\x1b[0m')
      }
    }
    
    ws.onclose = () => {
      console.log('WebSocket连接已关闭')
      if (terminal) {
        terminal.writeln('\x1b[33m[连接已断开，正在重连...]\x1b[0m')
      }
      // 延迟重连
      reconnectTimer = setTimeout(() => {
        connectWebSocket()
      }, reconnectDelay)
    }
  } catch (error) {
    console.error('创建WebSocket连接失败:', error)
    if (terminal) {
      terminal.writeln('\x1b[31m[无法连接到日志服务器]\x1b[0m')
    }
  }
}

const initTerminal = async () => {
  await nextTick()
  
  if (!terminalContainer.value) {
    return
  }
  
  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 13,
    fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Monaco, "Cascadia Code", "Roboto Mono", Consolas, "Courier New", monospace',
    fontWeight: '400',
    letterSpacing: 0.5,
    theme: {
      background: '#1e1e1e',
      foreground: '#ffffff',
      cursor: '#aeafad',
      selection: '#3a3d41',
      black: '#000000',
      red: '#cd3131',
      green: '#0dbc79',
      yellow: '#e5e510',
      blue: '#2472c8',
      magenta: '#bc3fbc',
      cyan: '#11a8cd',
      white: '#ffffff',
      brightBlack: '#666666',
      brightRed: '#f14c4c',
      brightGreen: '#23d18b',
      brightYellow: '#f5f543',
      brightBlue: '#3b8eea',
      brightMagenta: '#d670d6',
      brightCyan: '#29b8db',
      brightWhite: '#ffffff'
    },
    scrollback: 10000, // 最多保留10000行历史记录，超过会自动清理旧内容
    disableStdin: false // 启用stdin以监听键盘事件
  })
  
  fitAddon = new FitAddon()
  searchAddon = new SearchAddon()
  terminal.loadAddon(fitAddon)
  terminal.loadAddon(searchAddon)
  terminal.open(terminalContainer.value)
  
  // 初始调整大小
  fitAddon.fit()
  
  // 监听窗口大小变化
  const resizeObserver = new ResizeObserver(() => {
    if (fitAddon) {
      fitAddon.fit()
    }
  })
  resizeObserver.observe(terminalContainer.value)
  
  // 监听滚动事件，检测用户是否在底部
  // 使用setTimeout确保terminal.element已经渲染
  setTimeout(() => {
    const viewport = terminal?.element?.querySelector('.xterm-viewport') as HTMLElement
    if (viewport) {
      viewport.addEventListener('scroll', () => {
        if (terminal) {
          const scrollTop = viewport.scrollTop
          const scrollHeight = viewport.scrollHeight
          const clientHeight = viewport.clientHeight
          // 判断是否在底部（允许2px的误差）
          isUserAtBottom = scrollTop + clientHeight >= scrollHeight - 2
        }
      })
    }
  }, 100)
  
  // 监听键盘事件
  terminal.onData((data) => {
    // 回车键或Ctrl+M - 滚动到底部
    if (data === '\r' || data === '\n' || data === '\x0d' || data === '\x0a') {
      if (terminal && !showSearch.value) {
        terminal.scrollToBottom()
        isUserAtBottom = true
      }
    }
  })
  
  // 监听全局键盘事件，实现Ctrl+F搜索
  window.addEventListener('keydown', handleKeyDown)
  
  // 监听鼠标滚轮事件，检测用户是否在底部
  terminal.element?.addEventListener('wheel', () => {
    // 延迟检查，等待滚动完成
    setTimeout(() => {
      if (terminal) {
        const viewport = terminal.element?.querySelector('.xterm-viewport') as HTMLElement
        if (viewport) {
          const scrollTop = viewport.scrollTop
          const scrollHeight = viewport.scrollHeight
          const clientHeight = viewport.clientHeight
          isUserAtBottom = scrollTop + clientHeight >= scrollHeight - 1
        }
      }
    }, 100)
  })
  
  // 显示欢迎信息
  terminal.writeln('\x1b[36m=== 0E7 日志查看器 ===\x1b[0m')
  terminal.writeln('正在连接日志服务器...\r\n')
  terminal.writeln('\x1b[33m提示: 按回车键滚动到底部 | Ctrl+F 搜索日志\x1b[0m\r\n')
  // 滚动到底部
  terminal.scrollToBottom()
  isUserAtBottom = true
  
  // 连接WebSocket
  connectWebSocket()
}

// 搜索功能
const findNext = () => {
  if (searchAddon && searchTerm.value) {
    searchAddon.findNext(searchTerm.value, {
      regex: false,
      wholeWord: false,
      caseSensitive: false
    })
  }
}

const findPrevious = () => {
  if (searchAddon && searchTerm.value) {
    searchAddon.findPrevious(searchTerm.value, {
      regex: false,
      wholeWord: false,
      caseSensitive: false
    })
  }
}

const closeSearch = () => {
  showSearch.value = false
  searchTerm.value = ''
  if (searchAddon) {
    searchAddon.clearDecorations()
  }
}

onMounted(() => {
  initTerminal()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
  }
  if (ws) {
    ws.close()
  }
  if (terminal) {
    terminal.dispose()
  }
})
</script>

<style scoped>
.log-viewer {
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  background: #fff;
  border-radius: 6px;
  border: 1px solid #e6e8eb;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  overflow: hidden;
  position: relative;
}

.terminal-container {
  width: 100%;
  height: 100%;
  padding: 10px;
  box-sizing: border-box;
  background: #1e1e1e;
  border-radius: 4px;
}

/* 确保终端样式正确 */
:deep(.xterm) {
  height: 100%;
}

:deep(.xterm-viewport) {
  background-color: #1e1e1e !important;
}

:deep(.xterm-screen) {
  background-color: #1e1e1e !important;
}

/* 搜索框样式 */
.search-box {
  position: absolute;
  top: 30px;
  right: 30px;
  z-index: 1000;
  background: #fff;
  padding: 8px;
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  min-width: 400px;
}

.search-input {
  width: 100%;
}

.search-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 8px;
}

.search-hint kbd {
  background: #f5f7fa;
  border: 1px solid #dcdfe6;
  border-radius: 3px;
  padding: 2px 6px;
  font-size: 11px;
  font-family: monospace;
  margin: 0 2px;
  color: #606266;
}
</style>
