<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { ElNotification, ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'

interface FlowItem {
  f: string  // from: 'c' for client, 's' for server
  d: string  // data
  b: string  // base64
  t: number  // time
}

interface PcapDetail {
  id: number
  src_ip: string
  src_port: string
  dst_ip: string
  dst_port: string
  time: number
  duration: number
  num_packets: number
  blocked: string
  filename: string
  fingerprints: string
  suricata: string
  flow: string
  tags: string
  size: string
  created_at: string
  updated_at: string
}

const props = defineProps<{
  pcapId: number
}>()

const emit = defineEmits(['close'])

const pcapDetail = ref<PcapDetail | null>(null)
const flowData = ref<FlowItem[]>([])
const loading = ref(false)
const activeTab = ref('text')
// 为每个流量维护独立的选择状态
const flowSelections = ref<Map<number, { selectedByte: number, selectedRange: { start: number, end: number } | null }>>(new Map())

// 流量大小相关状态
const flowSize = ref<number>(0)
const showSizeWarning = ref(false)
const showDownloadOption = ref(false)
const flowSizeInfo = ref<{ size: number, path: string } | null>(null)

// 拖拽选择状态
const dragSelection = ref<{ flowIndex: number, startByte: number, isDragging: boolean } | null>(null)

// 计算属性：为每个flow项预计算十六进制行数据
const flowHexData = computed(() => {
  return flowData.value.map(flow => ({
    ...flow,
    hexRows: getHexRows(flow.d)
  }))
})

// 获取流量详情
const fetchPcapDetail = async () => {
  loading.value = true
  try {
    const formData = new FormData()
    formData.append('id', props.pcapId.toString())

    const response = await fetch('/webui/pcap_get_by_id', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success' && result.result) {
      pcapDetail.value = result.result
      
      // 获取flow文件信息并决定加载策略
      if (result.result.flow) {
        await fetchFlowInfo(result.result.flow)
      }
    } else {
      ElNotification({
        title: '获取失败',
        message: result.error || '获取流量详情失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('获取流量详情失败:', error)
    ElNotification({
      title: '获取失败',
      message: '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right'
    })
  } finally {
    loading.value = false
  }
}

// 获取flow文件信息
const fetchFlowInfo = async (flowPath: string) => {
  try {
    const formData = new FormData()
    formData.append('flow_path', flowPath)
    formData.append('i', 'true') // 添加信息参数

    const response = await fetch('/webui/flow_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const result = await response.json()
      if (result.message === 'success' && result.result) {
        flowSizeInfo.value = result.result
        flowSize.value = result.result.size
        
        // 根据文件大小决定显示策略
        const sizeKB = result.result.size / 1024
        
        if (sizeKB > 30) {
          // 大于30KB，显示下载选项
          showDownloadOption.value = true
          showSizeWarning.value = false
        } else if (sizeKB > 10) {
          // 10KB-30KB，显示警告但不默认加载
          showSizeWarning.value = true
          showDownloadOption.value = false
        } else {
          // 小于10KB，正常加载
          showSizeWarning.value = false
          showDownloadOption.value = false
          await fetchFlowData(flowPath)
        }
      }
    } else {
      console.error('获取flow文件信息失败:', response.status, response.statusText)
    }
  } catch (error) {
    console.error('获取flow文件信息失败:', error)
  }
}

// 获取flow数据
const fetchFlowData = async (flowPath: string) => {
  try {
    const formData = new FormData()
    formData.append('flow_path', flowPath)

    const response = await fetch('/webui/flow_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const result = await response.json()
      // 按时间排序流量数据
      flowData.value = (result || []).sort((a: FlowItem, b: FlowItem) => a.t - b.t)
    } else {
      console.error('获取flow数据失败:', response.status, response.statusText)
    }
  } catch (error) {
    console.error('获取flow数据失败:', error)
  }
}

// 强制加载流量数据
const forceLoadFlowData = async () => {
  if (pcapDetail.value?.flow) {
    showSizeWarning.value = false
    await fetchFlowData(pcapDetail.value.flow)
  }
}

// 下载流量文件
const downloadFlowFile = async () => {
  if (!pcapDetail.value?.flow || !flowSizeInfo.value) return
  
  try {
    const formData = new FormData()
    formData.append('flow_path', pcapDetail.value.flow)
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('d', 'true') // 添加下载参数

    const response = await fetch('/webui/flow_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `pcap_${props.pcapId}.json${pcapDetail.value.flow.endsWith('.gz') ? '.gz' : ''}`
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } else {
      console.error('下载flow文件失败:', response.status, response.statusText)
    }
  } catch (error) {
    console.error('下载flow文件失败:', error)
  }
}

// 格式化文件大小
const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化时间戳
const formatTimestamp = (timestamp: number) => {
  // 判断时间戳是秒级还是毫秒级
  // 如果时间戳大于 1000000000000，说明是毫秒级
  const isMilliseconds = timestamp > 1000000000000
  const date = new Date(isMilliseconds ? timestamp : timestamp * 1000)
  
  // 检查日期是否有效
  if (isNaN(date.getTime())) {
    return '无效时间'
  }
  
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }) + '.' + String(date.getMilliseconds()).padStart(3, '0')
}

// 解析标签
const parseTags = (tagsStr: string) => {
  try {
    if (!tagsStr || tagsStr === '[]') {
      return []
    }
    
    // 处理Unicode引号问题：将Unicode左右单引号替换为标准双引号
    let normalizedStr = tagsStr
      .replace(/[\u2018\u2019]/g, '"')  // 替换Unicode单引号为双引号
      .replace(/[\u201c\u201d]/g, '"')  // 替换Unicode双引号为标准双引号
    
    return JSON.parse(normalizedStr)
  } catch (error) {
    console.warn('解析标签失败:', tagsStr, error)
    return []
  }
}

// 获取标签颜色
const getTagType = (tag: string) => {
  const tagColors: { [key: string]: string } = {
    'FLAG-OUT': 'success',
    'FLAG-IN': 'warning',
    'SURICATA': 'danger',
    'ENEMY': 'info',
    'BLOCKED': 'danger',
    'RCE': 'danger',
    'MEME': 'warning',
    'SQLI': 'danger',
    'PHP-RCE': 'danger',
    'PATH TRAVERSAL': 'danger',
    'AUTH': 'warning',
    'CRYPTO': 'info',
    'PHP-LFI': 'danger',
    'SSRF': 'danger',
    'INJECTION': 'danger',
    'BOF': 'danger',
    'STARRED': 'primary'
  }
  // 确保返回有效的Element Plus标签类型
  const validTypes = ['primary', 'success', 'warning', 'danger', 'info']
  const type = tagColors[tag] || 'info'
  return validTypes.includes(type) ? type : 'info'
}

// 格式化文件大小
const formatSize = (sizeStr: string) => {
  const size = parseInt(sizeStr || '0')
  if (size === 0) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`
  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

// 将文本转换为十六进制
const textToHex = (text: string) => {
  return Array.from(text)
    .map(char => char.charCodeAt(0).toString(16).padStart(2, '0'))
    .join(' ')
}

// 将十六进制转换为文本
const hexToText = (hex: string) => {
  try {
    return hex.split(' ')
      .map(hexChar => String.fromCharCode(parseInt(hexChar, 16)))
      .join('')
  } catch {
    return hex
  }
}

// 高亮显示十六进制和文本的对应关系
const highlightHexText = (text: string) => {
  const hex = textToHex(text)
  const hexChars = hex.split(' ')
  const textChars = Array.from(text)
  
  return {
    hex: hexChars,
    text: textChars
  }
}

// 复制到剪贴板
const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 获取流量方向标识
const getFlowDirection = (from: string) => {
  return from === 'c' ? '客户端 → 服务器' : '服务器 → 客户端'
}

// 获取流量方向颜色
const getFlowDirectionColor = (from: string) => {
  return from === 'c' ? '#e1f5fe' : '#f3e5f5'
}

// 缓存十六进制行数据
const hexRowsCache = ref<Map<string, any[]>>(new Map())

// 将数据转换为十六进制编辑器格式（带缓存）
const getHexRows = (data: string) => {
  if (hexRowsCache.value.has(data)) {
    return hexRowsCache.value.get(data)
  }
  
  const bytes = new TextEncoder().encode(data)
  const rows = []
  
  for (let i = 0; i < bytes.length; i += 16) {
    const rowBytes = bytes.slice(i, i + 16)
    const hexBytes = Array.from(rowBytes).map(b => b.toString(16).padStart(2, '0').toUpperCase())
    const asciiChars = Array.from(rowBytes).map(b => {
      const char = String.fromCharCode(b)
      return (b >= 32 && b <= 126) ? char : '.'
    })
    
    // 确保每行都有16个字节位置，不足的用空字符串填充
    const paddedHexBytes = [...hexBytes]
    const paddedAsciiChars = [...asciiChars]
    
    while (paddedHexBytes.length < 16) {
      paddedHexBytes.push('')
      paddedAsciiChars.push('')
    }
    
    rows.push({
      offset: i.toString(16).padStart(4, '0').toUpperCase(),
      bytes: paddedHexBytes,
      ascii: paddedAsciiChars,
      originalLength: rowBytes.length // 保存原始字节数量
    })
  }
  
  // 缓存结果，但限制缓存大小
  if (hexRowsCache.value.size > 10) {
    const firstKey = hexRowsCache.value.keys().next().value
    if (firstKey) {
      hexRowsCache.value.delete(firstKey)
    }
  }
  hexRowsCache.value.set(data, rows)
  
  return rows
}

// 防抖函数
const debounce = (func: Function, wait: number) => {
  let timeout: ReturnType<typeof setTimeout>
  return function executedFunction(...args: any[]) {
    const later = () => {
      clearTimeout(timeout)
      func(...args)
    }
    clearTimeout(timeout)
    timeout = setTimeout(later, wait)
  }
}

// 获取流量的选择状态
const getFlowSelection = (flowIndex: number) => {
  if (!flowSelections.value.has(flowIndex)) {
    flowSelections.value.set(flowIndex, { selectedByte: -1, selectedRange: null })
  }
  return flowSelections.value.get(flowIndex)!
}

// 开始拖拽选择
const startDragSelection = (flowIndex: number, byteIndex: number) => {
  dragSelection.value = {
    flowIndex,
    startByte: byteIndex,
    isDragging: true
  }
  
  // 清除之前的选择
  const selection = getFlowSelection(flowIndex)
  selection.selectedByte = byteIndex
  selection.selectedRange = null
  flowSelections.value.set(flowIndex, { ...selection })
}

// 拖拽选择中
const dragSelectionUpdate = (flowIndex: number, byteIndex: number) => {
  if (!dragSelection.value || dragSelection.value.flowIndex !== flowIndex || !dragSelection.value.isDragging) {
    return
  }
  
  const selection = getFlowSelection(flowIndex)
  const start = Math.min(dragSelection.value.startByte, byteIndex)
  const end = Math.max(dragSelection.value.startByte, byteIndex)
  selection.selectedRange = { start, end }
  flowSelections.value.set(flowIndex, { ...selection })
}

// 结束拖拽选择
const endDragSelection = () => {
  dragSelection.value = null
}

// 选择字节（支持批量选择）
const selectByte = (flowIndex: number, byteIndex: number, event?: MouseEvent) => {
  const selection = getFlowSelection(flowIndex)
  
  if (event && event.altKey && selection.selectedByte !== -1) {
    // Alt + 点击：选择范围
    const start = Math.min(selection.selectedByte, byteIndex)
    const end = Math.max(selection.selectedByte, byteIndex)
    selection.selectedRange = { start, end }
  } else if (event && event.ctrlKey && selection.selectedRange) {
    // Ctrl + 点击：扩展选择范围
    const currentRange = selection.selectedRange
    const newStart = Math.min(currentRange.start, byteIndex)
    const newEnd = Math.max(currentRange.end, byteIndex)
    selection.selectedRange = { start: newStart, end: newEnd }
  } else {
    // 普通点击：选择单个字节
    selection.selectedByte = byteIndex
    selection.selectedRange = null
  }
  
  // 触发响应式更新
  flowSelections.value.set(flowIndex, { ...selection })
}

// 清除选择
const clearSelection = (flowIndex: number) => {
  flowSelections.value.set(flowIndex, { selectedByte: -1, selectedRange: null })
}

// 检查字节是否被选中
const isByteSelected = (flowIndex: number, byteIndex: number) => {
  const selection = getFlowSelection(flowIndex)
  
  if (selection.selectedRange) {
    return byteIndex >= selection.selectedRange.start && byteIndex <= selection.selectedRange.end
  }
  
  return selection.selectedByte === byteIndex
}

// 复制选中的字节数据
const copySelectedBytes = async (flowIndex: number) => {
  const flow = flowData.value[flowIndex]
  const selection = getFlowSelection(flowIndex)
  
  if (selection.selectedByte === -1 && !selection.selectedRange) {
    ElMessage.warning('请先选择要复制的字节')
    return
  }
  
  try {
    let selectedData = ''
    let selectedHex = ''
    
    if (selection.selectedRange) {
      // 复制范围
      const start = selection.selectedRange.start
      const end = selection.selectedRange.end
      selectedData = flow.d.slice(start, end + 1)
      selectedHex = textToHex(selectedData)
    } else {
      // 复制单个字节
      selectedData = flow.d[selection.selectedByte]
      selectedHex = textToHex(selectedData)
    }
    
    const copyText = `原始数据: ${selectedData}\n十六进制: ${selectedHex}`
    await navigator.clipboard.writeText(copyText)
    ElMessage.success('已复制选中字节到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// HTML语法高亮
const highlightHTML = (text: string) => {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/&lt;(\/?[a-zA-Z][^&gt;]*?)&gt;/g, '<span class="html-tag">&lt;$1&gt;</span>')
    .replace(/&lt;!--[\s\S]*?--&gt;/g, '<span class="html-comment">&lt;!--$&--&gt;</span>')
    .replace(/&lt;script[\s\S]*?&lt;\/script&gt;/gi, '<span class="html-script">$&</span>')
    .replace(/&lt;style[\s\S]*?&lt;\/style&gt;/gi, '<span class="html-style">$&</span>')
    .replace(/(\w+)=&quot;([^&quot;]*)&quot;/g, '<span class="html-attr">$1</span>=<span class="html-value">&quot;$2&quot;</span>')
    .replace(/(\w+)=&apos;([^&apos;]*)&apos;/g, '<span class="html-attr">$1</span>=<span class="html-value">&apos;$2&apos;</span>')
    .replace(/(\w+)=([^&gt;\s]+)/g, '<span class="html-attr">$1</span>=<span class="html-value">$2</span>')
}

onMounted(() => {
  fetchPcapDetail()
  
  // 添加全局鼠标事件监听
  document.addEventListener('mouseup', endDragSelection)
  document.addEventListener('mouseleave', endDragSelection)
})

// 组件卸载时清理事件监听
onUnmounted(() => {
  document.removeEventListener('mouseup', endDragSelection)
  document.removeEventListener('mouseleave', endDragSelection)
})
</script>

<template>
  <div class="pcap-detail-container" v-loading="loading">
    <div v-if="pcapDetail" class="detail-content">
      <!-- 基本信息 -->
      <div class="info-section">
        <h2 class="section-title">流量基本信息</h2>
        <div class="info-grid">
          <div class="info-item">
            <label>流量ID:</label>
            <span>{{ pcapDetail.id }}</span>
          </div>
          <div class="info-item">
            <label>源地址:</label>
            <span>{{ pcapDetail.src_ip }}:{{ pcapDetail.src_port }}</span>
          </div>
          <div class="info-item">
            <label>目标地址:</label>
            <span>{{ pcapDetail.dst_ip }}:{{ pcapDetail.dst_port }}</span>
          </div>
          <div class="info-item">
            <label>数据包数:</label>
            <span>{{ pcapDetail.num_packets }}</span>
          </div>
          <div class="info-item">
            <label>持续时间:</label>
            <span>{{ pcapDetail.duration }}ms</span>
          </div>
          <div class="info-item">
            <label>数据大小:</label>
            <span>{{ formatSize(pcapDetail.size) }}</span>
          </div>
          <div class="info-item">
            <label>状态:</label>
            <el-tag :type="pcapDetail.blocked === 'true' ? 'danger' : 'success'">
              {{ pcapDetail.blocked === 'true' ? '已阻止' : '允许' }}
            </el-tag>
          </div>
          <div class="info-item">
            <label>文件:</label>
            <span>{{ pcapDetail.filename || '未知' }}</span>
          </div>
          <div class="info-item">
            <label>时间:</label>
            <span>{{ formatTimestamp(pcapDetail.time) }}</span>
          </div>
        </div>
      </div>

      <!-- 标签信息 -->
      <div class="tags-section" v-if="parseTags(pcapDetail.tags).length > 0">
        <h3 class="section-title">标签</h3>
        <div class="tags-container">
          <el-tag
            v-for="tag in parseTags(pcapDetail.tags)"
            :key="tag"
            :type="getTagType(tag)"
            size="small"
            class="tag-item"
          >
            {{ tag }}
          </el-tag>
        </div>
      </div>

      <!-- 流量大小警告 -->
      <div class="flow-warning-section" v-if="showSizeWarning">
        <el-alert
          title="流量文件较大"
          type="warning"
          :description="`流量文件大小为 ${formatFileSize(flowSize)}，加载可能需要较长时间。是否继续加载？`"
          show-icon
          :closable="false"
        >
          <template #default>
            <div class="warning-actions">
              <el-button type="primary" @click="forceLoadFlowData">继续加载</el-button>
              <el-button @click="showSizeWarning = false">取消</el-button>
            </div>
          </template>
        </el-alert>
      </div>

      <!-- 流量下载选项 -->
      <div class="flow-download-section" v-if="showDownloadOption">
        <el-alert
          title="流量文件过大"
          type="info"
          :description="`流量文件大小为 ${formatFileSize(flowSize)}，建议下载到本地查看。`"
          show-icon
          :closable="false"
        >
          <template #default>
            <div class="download-actions">
              <el-button type="primary" @click="downloadFlowFile">
                <el-icon><Download /></el-icon>
                下载流量文件
              </el-button>
              <el-button @click="forceLoadFlowData">仍要在线查看</el-button>
            </div>
          </template>
        </el-alert>
      </div>

      <!-- 流量数据 -->
      <div class="flow-section" v-if="flowData.length > 0">
        <h3 class="section-title">流量数据</h3>
        <div class="flow-list">
          <div
            v-for="(flow, index) in flowHexData"
            :key="index"
            class="flow-item"
            :style="{ backgroundColor: getFlowDirectionColor(flow.f) }"
          >
            <div class="flow-header">
              <div class="flow-direction">
                <el-icon v-if="flow.f === 'c'"><ArrowRight /></el-icon>
                <el-icon v-else><ArrowLeft /></el-icon>
                <span>{{ getFlowDirection(flow.f) }}</span>
                <span class="flow-time">{{ formatTimestamp(flow.t) }}</span>
              </div>
              <div class="flow-actions">
                <!-- 只在对比显示标签页中显示选择相关按钮 -->
                <template v-if="activeTab === 'compare'">
                  <el-button 
                    size="small" 
                    @click="clearSelection(index)"
                    :disabled="getFlowSelection(index).selectedByte === -1 && !getFlowSelection(index).selectedRange"
                  >
                    <el-icon><Close /></el-icon>
                    清除选择
                  </el-button>
                  <el-button 
                    size="small" 
                    @click="copySelectedBytes(index)"
                    :disabled="getFlowSelection(index).selectedByte === -1 && !getFlowSelection(index).selectedRange"
                  >
                    <el-icon><CopyDocument /></el-icon>
                    复制选中
                  </el-button>
                </template>
                <!-- 复制全部按钮在所有标签页都显示 -->
                <el-button size="small" @click="copyToClipboard(flow.d)">
                  <el-icon><CopyDocument /></el-icon>
                  复制全部
                </el-button>
              </div>
            </div>
            
            <div class="flow-content">
              <div class="content-tabs">
                <div class="tab-buttons">
                  <button 
                    :class="['tab-btn', { active: activeTab === 'text' }]"
                    @click="activeTab = 'text'"
                  >
                    文本
                  </button>
                  <button 
                    :class="['tab-btn', { active: activeTab === 'hex' }]"
                    @click="activeTab = 'hex'"
                  >
                    十六进制
                  </button>
                  <button 
                    :class="['tab-btn', { active: activeTab === 'compare' }]"
                    @click="activeTab = 'compare'"
                  >
                    对比显示
                  </button>
                </div>
                
                <!-- 选择信息显示 -->
                <div v-if="activeTab === 'compare'" class="selection-info">
                  <div class="selection-status">
                    <span v-if="getFlowSelection(index).selectedByte !== -1">
                      已选择字节: {{ getFlowSelection(index).selectedByte }}
                    </span>
                    <span v-else-if="getFlowSelection(index).selectedRange">
                      已选择范围: {{ getFlowSelection(index).selectedRange?.start }} - {{ getFlowSelection(index).selectedRange?.end }}
                      ({{ (getFlowSelection(index).selectedRange?.end || 0) - (getFlowSelection(index).selectedRange?.start || 0) + 1 }} 字节)
                    </span>
                    <span v-else class="selection-hint">
                      点击选择字节 | 拖拽选择范围 | Alt+点击选择范围 | Ctrl+点击扩展范围
                    </span>
                  </div>
                </div>
                
                <div class="tab-content">
                  <div v-if="activeTab === 'text'" class="content-display">
                    <div class="text-content" v-html="highlightHTML(flow.d)"></div>
                  </div>
                  
                  <div v-if="activeTab === 'hex'" class="content-display">
                    <div class="hex-content">{{ textToHex(flow.d) }}</div>
                  </div>
                  
                  <div v-if="activeTab === 'compare'" class="hex-editor-display">
                    <div class="hex-editor">
                      <div class="hex-header">
                        <div class="offset-column">Offset</div>
                        <div class="hex-column">Hexadecimal</div>
                        <div class="ascii-column">ASCII</div>
                      </div>
                      <div class="hex-content" ref="hexContent">
                        <div 
                          v-for="(row, rowIndex) in flow.hexRows" 
                          :key="rowIndex" 
                          class="hex-row"
                        >
                          <div class="offset-cell">{{ row.offset }}</div>
                          <div class="hex-cells">
                            <span
                              v-for="(byte, byteIndex) in row.bytes"
                              :key="byteIndex"
                              :class="['hex-byte', { 
                                highlighted: isByteSelected(index, rowIndex * 16 + byteIndex),
                                'hex-byte-spacer': byteIndex % 8 === 7 && byteIndex < 15,
                                'empty-byte': !byte && byteIndex >= row.originalLength
                              }]"
                              @mousedown="byte && startDragSelection(index, rowIndex * 16 + byteIndex)"
                              @mouseenter="byte && dragSelectionUpdate(index, rowIndex * 16 + byteIndex)"
                              @click="byte && selectByte(index, rowIndex * 16 + byteIndex, $event)"
                              :data-index="rowIndex * 16 + byteIndex"
                            >
                              {{ byte }}
                            </span>
                          </div>
                          <div class="ascii-cells">
                            <span
                              v-for="(char, charIndex) in row.ascii"
                              :key="charIndex"
                              :class="['ascii-char', { 
                                highlighted: isByteSelected(index, rowIndex * 16 + charIndex),
                                'ascii-char-spacer': charIndex % 8 === 7 && charIndex < 15,
                                'empty-char': !char && charIndex >= row.originalLength
                              }]"
                              @mousedown="char && startDragSelection(index, rowIndex * 16 + charIndex)"
                              @mouseenter="char && dragSelectionUpdate(index, rowIndex * 16 + charIndex)"
                              @click="char && selectByte(index, rowIndex * 16 + charIndex, $event)"
                              :data-index="rowIndex * 16 + charIndex"
                            >
                              {{ char }}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 无流量数据提示 -->
      <div v-else class="no-data">
        <el-empty description="暂无流量数据" />
      </div>
    </div>

    <!-- 关闭按钮 -->
    <div class="close-section">
      <el-button @click="emit('close')">
        <el-icon><Close /></el-icon>
        关闭
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.pcap-detail-container {
  background: #fff;
  border-radius: 6px;
  border: 1px solid #e6e8eb;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.detail-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.section-title {
  color: #303133;
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 15px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid #409eff;
}

.info-section {
  margin-bottom: 20px;
  flex-shrink: 0;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 15px;
}

.info-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 4px;
}

.info-item label {
  font-weight: 600;
  color: #606266;
  margin-right: 8px;
  min-width: 80px;
}

.info-item span {
  color: #303133;
  font-family: 'Courier New', monospace;
}

.tags-section {
  margin-bottom: 20px;
  flex-shrink: 0;
}

.tags-container {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.tag-item {
  margin: 0;
}

.flow-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.flow-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 15px;
  overflow-y: auto;
  min-height: 0;
}

.flow-item {
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  overflow: hidden;
  transition: all 0.3s;
  flex-shrink: 0;
  max-height: 600px;
  display: flex;
  flex-direction: column;
}

.flow-item:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.flow-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 15px;
  background: rgba(255, 255, 255, 0.8);
  border-bottom: 1px solid #e6e8eb;
}

.flow-direction {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: #303133;
}

.flow-time {
  font-size: 12px;
  color: #909399;
  font-weight: normal;
  margin-left: 10px;
}

.flow-actions {
  display: flex;
  gap: 8px;
}

.flow-content {
  padding: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.content-tabs {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.tab-buttons {
  display: flex;
  background: #f5f7fa;
  border-bottom: 1px solid #e6e8eb;
  padding: 0 15px;
}

.tab-btn {
  padding: 8px 16px;
  border: none;
  background: transparent;
  color: #606266;
  cursor: pointer;
  font-size: 14px;
  border-bottom: 2px solid transparent;
  transition: all 0.3s;
}

.tab-btn:hover {
  color: #409eff;
  background: rgba(64, 158, 255, 0.1);
}

.tab-btn.active {
  color: #409eff;
  border-bottom-color: #409eff;
  background: #fff;
}

.selection-info {
  padding: 8px 15px;
  background: #f0f9ff;
  border-bottom: 1px solid #e6e8eb;
  font-size: 12px;
}

.selection-status {
  display: flex;
  align-items: center;
  gap: 10px;
}

.selection-hint {
  color: #909399;
  font-style: italic;
}

.tab-content {
  flex: 1;
  overflow: hidden;
}

.content-display {
  padding: 15px;
  background: #fafafa;
  border-radius: 4px;
  margin: 10px;
  flex: 1;
  overflow-y: auto;
  overflow-x: auto;
  max-height: 400px;
  min-height: 200px;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
}

.content-display::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.content-display::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 4px;
}

.content-display::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 4px;
}

.content-display::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

.text-content, .hex-content {
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.4;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-all;
}

/* HTML语法高亮样式 */
.html-tag {
  color: #800080;
  font-weight: bold;
}

.html-attr {
  color: #ff6600;
  font-weight: bold;
}

.html-value {
  color: #008000;
}

.html-comment {
  color: #808080;
  font-style: italic;
}

.html-script {
  background: #f0f8ff;
  padding: 2px 4px;
  border-radius: 2px;
}

.html-style {
  background: #fff0f5;
  padding: 2px 4px;
  border-radius: 2px;
}

.hex-editor-display {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.hex-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #fafafa;
  color: #303133;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  overflow: hidden;
  border-radius: 4px;
  margin: 10px;
  min-height: 300px;
  max-height: 500px;
}

.hex-header {
  display: flex;
  background: #f5f7fa;
  border-bottom: 1px solid #e6e8eb;
  padding: 8px 0;
  font-weight: bold;
  color: #606266;
}

.offset-column {
  width: 70px;
  padding: 0 6px 0 12px;
  text-align: left;
  flex-shrink: 0;
}

.hex-column {
  flex: 1;
  padding: 0 6px 0 0;
  text-align: center;
  max-width: 500px;
}

.ascii-column {
  width: 240px;
  padding: 0 6px;
  text-align: center;
}

.hex-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: auto;
  min-height: 0;
  max-height: 400px;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
}

.hex-content::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.hex-content::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 4px;
}

.hex-content::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 4px;
}

.hex-content::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

.hex-row {
  display: flex;
  align-items: center;
  padding: 2px 0;
  border-bottom: 1px solid #e6e8eb;
}

.hex-row:hover {
  background: #f0f2f5;
}

.offset-cell {
  width: 70px;
  padding: 0 6px 0 12px;
  text-align: left;
  color: #409eff;
  font-weight: bold;
  flex-shrink: 0;
}

.hex-cells {
  flex: 1;
  padding: 0 6px 0 0;
  display: flex;
  justify-content: flex-start;
  gap: 2px;
  max-width: 500px;
  flex-wrap: wrap;
}

.hex-byte {
  display: inline-block;
  padding: 2px 4px;
  cursor: pointer;
  border-radius: 2px;
  transition: all 0.2s;
  min-width: 20px;
  text-align: center;
}

.hex-byte:hover {
  background: #e6f7ff;
}

.hex-byte.highlighted {
  background: #409eff;
  color: #ffffff;
}

.hex-byte-spacer {
  margin-right: 6px;
}

.empty-byte {
  visibility: hidden;
  pointer-events: none;
}

.empty-char {
  visibility: hidden;
  pointer-events: none;
}

.ascii-cells {
  width: 240px;
  padding: 0 6px;
  display: flex;
  justify-content: flex-start;
  gap: 1px;
  flex-wrap: nowrap;
}

.ascii-char {
  display: inline-block;
  padding: 2px 3px;
  cursor: pointer;
  border-radius: 2px;
  transition: all 0.2s;
  min-width: 14px;
  text-align: center;
}

.ascii-char:hover {
  background: #e6f7ff;
}

.ascii-char.highlighted {
  background: #409eff;
  color: #ffffff;
}

.ascii-char-spacer {
  margin-right: 6px;
}

.compare-display {
  display: flex;
  gap: 20px;
  padding: 15px;
  background: #fafafa;
  margin: 10px;
  border-radius: 4px;
  flex: 1;
  overflow-y: auto;
}

.compare-section {
  flex: 1;
}

.compare-section h4 {
  margin: 0 0 10px 0;
  color: #606266;
  font-size: 14px;
  font-weight: 600;
}

.text-lines, .hex-lines {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.2;
  max-height: 400px;
  overflow-y: auto;
  overflow-x: auto;
}

.text-char, .hex-char {
  display: inline-block;
  padding: 2px 4px;
  border-radius: 2px;
  background: #fff;
  border: 1px solid #e6e8eb;
  min-width: 20px;
  text-align: center;
  transition: all 0.2s;
}

.text-char:hover, .hex-char:hover {
  background: #409eff;
  color: #fff;
  border-color: #409eff;
}

.no-data {
  text-align: center;
  padding: 40px 0;
}

.close-section {
  display: flex;
  justify-content: center;
  padding-top: 20px;
  border-top: 1px solid #e6e8eb;
}

/* 流量大小警告和下载选项样式 */
.flow-warning-section,
.flow-download-section {
  margin: 20px 0;
}

.warning-actions,
.download-actions {
  margin-top: 12px;
  display: flex;
  gap: 12px;
}

.warning-actions .el-button,
.download-actions .el-button {
  margin: 0;
}

</style>
