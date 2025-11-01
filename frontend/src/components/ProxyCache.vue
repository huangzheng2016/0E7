<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { DocumentCopy } from '@element-plus/icons-vue'

interface CacheEntryMeta {
  key: string
  method: string
  url: string
  bodyHash: string
  status: string
  cachedAt: string
  expiresAt: string
  staleUntil: string
  ttlSeconds: number
  statusCode: number
  hits?: number
}

const loading = ref(false)
const rows = ref<CacheEntryMeta[]>([])
const auto = ref(false)
const intervalSec = ref(5)
let timer: any = null
const origin = typeof window !== 'undefined' ? window.location.origin : ''
const proxyPattern = ref(`curl ${origin}/proxy/{0s,5s,5m,1h}/{url/https://0e7.cn}`)

async function fetchList() {
  try {
    loading.value = true
    const resp = await fetch('/webui/proxy_cache_list', { method: 'POST' })
    const data = await resp.json()
    if (!data || data.success !== true) {
      throw new Error('请求失败')
    }
    // 直接渲染后端返回；时间字段转可读
    const newRows: CacheEntryMeta[] = (data.data || []).map((x: any) => ({
      key: x.Key || x.key,
      method: x.Method || x.method,
      url: x.URL || x.url,
      bodyHash: x.BodyHash || x.bodyHash,
      status: x.Status || x.status,
      cachedAt: fmtTime(x.CachedAt || x.cachedAt),
      expiresAt: fmtTime(x.ExpiresAt || x.expiresAt),
      staleUntil: fmtTime(x.StaleUntil || x.staleUntil),
      ttlSeconds: toSeconds(x.TTL ?? x.ttl),
      statusCode: x.StatusCode ?? x.statusCode ?? 0,
      hits: Number(x.hits ?? x.Hits ?? 0)
    }))
    // 保持数组引用不变，减少表格重绘闪烁
    rows.value.splice(0, rows.value.length, ...newRows)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function fmtTime(v: any): string {
  if (!v) return ''
  try {
    const d = new Date(v)
    if (isNaN(d.getTime())) return String(v)
    return d.toLocaleString()
  } catch {
    return String(v)
  }
}

function toSeconds(v: any): number {
  if (v == null) return 0
  if (typeof v === 'number') {
    // 兼容后端可能返回的纳秒数
    return Math.round((v / 1e9) * 1000) / 1000
  }
  if (typeof v === 'string') {
    const m = v.trim().match(/^(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)?$/i)
    if (!m) return Number(v) || 0
    const num = parseFloat(m[1])
    const unit = (m[2] || 's').toLowerCase()
    const factor: Record<string, number> = {
      ns: 1e-9,
      us: 1e-6,
      'µs': 1e-6,
      ms: 1e-3,
      s: 1,
      m: 60,
      h: 3600,
    }
    const f = factor[unit] ?? 1
    return Math.round(num * f * 1000) / 1000
  }
  return 0
}

function toggleAuto() {
  if (auto.value) {
    startTimer()
  } else {
    stopTimer()
  }
}

function startTimer() {
  stopTimer()
  if (intervalSec.value <= 0) return
  timer = setInterval(fetchList, intervalSec.value * 1000)
}

function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(() => {
  fetchList()
  if (auto.value) startTimer()
})

onUnmounted(() => {
  stopTimer()
})

// 复制 URL
const copyURL = (url: string) => {
  navigator.clipboard.writeText(url).then(() => {
    ElNotification({
      title: '成功',
      message: 'URL 已复制到剪贴板',
      type: 'success'
    })
  }).catch(err => {
    console.error('复制失败:', err)
    ElNotification({
      title: '错误',
      message: '复制失败',
      type: 'error'
    })
  })
}
</script>

<template>
  <div class="proxy-cache">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" :loading="loading" @click="fetchList">刷新</el-button>
        <el-switch v-model="auto" @change="toggleAuto" active-text="自动刷新" />
        <el-input-number v-model="intervalSec" :min="1" :max="300" @change="startTimer" />
      </div>
      <div class="toolbar-tip">
        代理示例：<strong>{{ proxyPattern }}</strong>
      </div>
    </div>

    <el-table :data="rows" height="calc(100% - 56px)" stripe>
      <el-table-column prop="method" label="方法" width="90" />
      <el-table-column prop="statusCode" label="状态" width="90" />
      <el-table-column prop="status" label="缓存状态" width="110" />
      <el-table-column prop="ttlSeconds" label="TTL(秒)" width="120" />
      <el-table-column prop="cachedAt" label="缓存时间" width="180" />
      <el-table-column prop="expiresAt" label="过期时间" width="180" />
      <el-table-column prop="staleUntil" label="保留至" width="180" />
      <el-table-column prop="hits" label="命中" width="90" />
      <el-table-column prop="url" label="URL">
        <template #default="{ row }">
          <div class="url-cell">
            <code class="url-text">{{ row.url }}</code>
            <el-button 
              type="text" 
              size="small"
              @click="copyURL(row.url)"
              class="copy-btn"
            >
              <el-icon><DocumentCopy /></el-icon>
              复制
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.proxy-cache {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.toolbar-tip {
  color: #606266;
  font-size: 12px;
  line-height: 1.4;
  text-align: right;
}

.url-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.url-text {
  flex: 1;
  background: #f5f7fa;
  padding: 4px 8px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  color: #409eff;
  word-break: break-all;
}

.copy-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
