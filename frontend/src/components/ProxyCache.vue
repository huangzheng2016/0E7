<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { DocumentCopy, Plus } from '@element-plus/icons-vue'

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

// 构造代理 URL 对话框相关
const showProxyBuilderDialog = ref(false)
const builderTTL = ref('5s')
const builderTargetURL = ref('0e7.cn')
const ttlOptions = [
  { label: '不缓存 (0s)', value: '0s' },
  { label: '5秒', value: '5s' },
  { label: '30秒', value: '30s' },
  { label: '1分钟', value: '1m' },
  { label: '5分钟', value: '5m' },
  { label: '10分钟', value: '10m' },
  { label: '30分钟', value: '30m' },
  { label: '1小时', value: '1h' },
  { label: '2小时', value: '2h' },
  { label: '6小时', value: '6h' },
  { label: '12小时', value: '12h' },
  { label: '24小时', value: '24h' }
]

// 实时生成的代理 URL
const generatedProxyURL = computed(() => {
  if (!builderTargetURL.value.trim()) {
    return ''
  }
  // 直接使用用户输入的目标 URL，不自动添加协议
  const targetURL = builderTargetURL.value.trim()
  return `${origin}/proxy/${builderTTL.value}/${encodeURIComponent(targetURL)}`
})

// 生成的完整 curl 命令
const generatedCurlCommand = computed(() => {
  if (!generatedProxyURL.value) {
    return ''
  }
  return `curl ${generatedProxyURL.value}`
})

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
    if (!m || !m[1]) return Number(v) || 0
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

// 打开代理 URL 构造对话框
const openProxyBuilder = () => {
  builderTTL.value = '5s'
  builderTargetURL.value = '0e7.cn'
  showProxyBuilderDialog.value = true
}

// 复制代理 URL
const copyProxyURL = () => {
  if (generatedProxyURL.value) {
    copyURL(generatedProxyURL.value)
  }
}

// 复制 curl 命令
const copyCurlCommand = () => {
  if (generatedCurlCommand.value) {
    copyURL(generatedCurlCommand.value)
  }
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
      <div class="toolbar-right">
        <el-button 
          type="success"
          @click="openProxyBuilder"
        >
          <el-icon><Plus /></el-icon>
          构造
        </el-button>
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

    <!-- 代理 URL 构造对话框 -->
    <el-dialog
      v-model="showProxyBuilderDialog"
      title="构造代理 URL"
      width="600px"
    >
      <el-form label-width="100px">
        <el-form-item label="TTL 时间">
          <el-select v-model="builderTTL" style="width: 100%">
            <el-option
              v-for="option in ttlOptions"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="目标 URL">
          <el-input
            v-model="builderTargetURL"
            placeholder="请输入目标 URL，例如: https://example.com"
          />
          <div style="margin-top: 4px; font-size: 12px; color: #909399;">
            请输入完整的目标 URL（包括 http:// 或 https://）
          </div>
        </el-form-item>

        <el-form-item label="代理 URL">
          <div class="generated-url-container">
            <code class="generated-url">{{ generatedProxyURL || '请输入目标 URL' }}</code>
            <el-button
              v-if="generatedProxyURL"
              type="text"
              size="small"
              @click="copyProxyURL"
              class="copy-btn"
            >
              <el-icon><DocumentCopy /></el-icon>
              复制 URL
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="curl 命令">
          <div class="generated-url-container">
            <code class="generated-url">{{ generatedCurlCommand || '请输入目标 URL' }}</code>
            <el-button
              v-if="generatedCurlCommand"
              type="text"
              size="small"
              @click="copyCurlCommand"
              class="copy-btn"
            >
              <el-icon><DocumentCopy /></el-icon>
              复制命令
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showProxyBuilderDialog = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>
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
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
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

.generated-url-container {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.generated-url {
  flex: 1;
  background: #f5f7fa;
  padding: 8px 12px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  color: #303133;
  word-break: break-all;
  display: block;
  min-height: 20px;
  line-height: 1.5;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
