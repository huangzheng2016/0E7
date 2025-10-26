<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
import { ElNotification, ElMessage, ElMessageBox } from 'element-plus'
import { Download, CopyDocument, ArrowDown, InfoFilled } from '@element-plus/icons-vue'
import { Codemirror } from 'vue-codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

interface FlowItem {
  f: string  // from: 'c' for client, 's' for server
  b: string  // base64 data
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
  flow_file: string
  pcap_file: string
  tags: string
  size: number
  created_at: string
  updated_at: string
}

const props = defineProps<{
  pcapId: number
  searchKeyword?: string
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
const flowSizeInfo = ref<{ raw_size?: number, flow_size?: number, parsed_size?: number } | null>(null)

// 拖拽选择状态
const dragSelection = ref<{ flowIndex: number, startByte: number, isDragging: boolean } | null>(null)

// 代码生成相关状态
const codeGenerationLoading = ref(false)
const showCodeDialog = ref(false)
const generatedCode = ref('')
const codeTitle = ref('')
const codeMirrorRef = ref()

// 解码base64数据的辅助函数
const decodeBase64 = (b64: string): string => {
  try {
    // 使用decodeURIComponent和escape来处理UTF-8字符
    return decodeURIComponent(escape(atob(b64)))
  } catch (error) {
    console.error('Failed to decode base64:', error)
    // 如果解码失败，尝试直接使用atob
    try {
      return atob(b64)
    } catch (e) {
      return '' // 如果都失败了，返回空字符串
    }
  }
}

// 计算属性：为每个flow项预计算十六进制行数据
const flowHexData = computed(() => {
  return flowData.value.map(flow => ({
    ...flow,
    hexRows: getHexRows(decodeBase64(flow.b))
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
      if (result.result.flow_file) {
        await fetchFlowInfo()
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
const fetchFlowInfo = async () => {
  try {
    const formData = new FormData()
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('type', 'parsed')
    formData.append('i', 'true') // 添加信息参数

    const response = await fetch('/webui/pcap_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const result = await response.json()
      if (result.message === 'success' && result.result) {
        flowSizeInfo.value = result.result
        flowSize.value = result.result.parsed_size || 0
        
        // 根据文件大小决定显示策略
        const sizeKB = flowSize.value / 1024
        
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
          await fetchFlowData()
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
const fetchFlowData = async () => {
  try {
    const formData = new FormData()
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('type', 'parsed')

    const response = await fetch('/webui/pcap_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const text = await response.text()
      try {
        const result = JSON.parse(text)
        console.log('pcap_download 返回的数据:', result)
        console.log('数据类型:', Array.isArray(result) ? '数组' : typeof result)
        console.log('数据长度:', Array.isArray(result) ? result.length : 'N/A')
        
        // 从返回的对象中提取Flow数组，然后按时间排序
        const flowArray = result.Flow || []
        flowData.value = flowArray.sort((a: FlowItem, b: FlowItem) => a.t - b.t)
        console.log('设置后的 flowData:', flowData.value)
        console.log('flowData 长度:', flowData.value.length)
      } catch (error) {
        console.error('解析flow数据失败:', error)
        console.error('原始数据:', text.substring(0, 200) + '...')
      }
    } else {
      console.error('获取flow数据失败:', response.status, response.statusText)
    }
  } catch (error) {
    console.error('获取flow数据失败:', error)
  }
}

// 强制加载流量数据
const forceLoadFlowData = async () => {
  if (pcapDetail.value?.flow_file) {
    showSizeWarning.value = false
    showDownloadOption.value = false
    await fetchFlowData()
  }
}


// 下载原始文件（未解析的原始pcap文件）
const downloadOriginalFile = async () => {
  if (!pcapDetail.value) return
  
  // 获取原始文件大小
  let fileSize = '未知大小'
  if (flowSizeInfo.value && flowSizeInfo.value.raw_size) {
    fileSize = formatFileSize(flowSizeInfo.value.raw_size)
  } else if (pcapDetail.value.size) {
    fileSize = formatFileSize(pcapDetail.value.size)
  }
  
  // 二次确认
  try {
    await ElMessageBox.confirm(
      `原始文件大小为 ${fileSize}，确定要下载吗？`,
      '确认下载',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return // 用户取消
  }
  
  try {
    const formData = new FormData()
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('type', 'raw')
    formData.append('d', 'true')

    const response = await fetch('/webui/pcap_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `raw_${props.pcapId}.pcap`
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } else {
      console.error('下载原始文件失败:', response.status, response.statusText)
      ElMessage.error('下载原始文件失败')
    }
  } catch (error) {
    console.error('下载原始文件失败:', error)
    ElMessage.error('下载原始文件失败')
  }
}

// 下载pcap文件
const downloadPcapFile = async (type: 'original' | 'parsed') => {
  if (!pcapDetail.value) return
  
  try {
    const formData = new FormData()
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('type', type)

    const response = await fetch('/webui/pcap_download', {
      method: 'POST',
      body: formData
    })
    
    if (response.ok) {
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      const fileName = type === 'original' ? `flow_${props.pcapId}.pcap` : `parsed_${props.pcapId}.json`
      a.download = fileName
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } else {
      console.error(`下载${type}文件失败:`, response.status, response.statusText)
      ElMessage.error(`下载${type === 'original' ? '流量' : '解析'}文件失败`)
    }
  } catch (error) {
    console.error(`下载${type}文件失败:`, error)
    ElMessage.error(`下载${type === 'original' ? '流量' : '解析'}文件失败`)
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
const formatSize = (size: number) => {
  if (size === 0) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(2)} MB`
  return `${(size / (1024 * 1024 * 1024)).toFixed(2)} GB`
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

// 生成代码
const generateCode = async (templateType: 'requests' | 'pwntools' | 'curl') => {
  if (!pcapDetail.value || flowData.value.length === 0) {
    ElMessage.warning('没有可用的流量数据')
    return
  }

  codeGenerationLoading.value = true
  try {
    const formData = new FormData()
    formData.append('pcap_id', props.pcapId.toString())
    formData.append('template', templateType)
    formData.append('flow_data', JSON.stringify(flowData.value))

    const response = await fetch('/webui/pcap_generate_code', {
      method: 'POST',
      body: formData
    })
    
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }
    
    const result = await response.json()
    
    if (result.message === 'success' && result.result) {
      const code = result.result.code
      
      // 尝试直接复制到剪贴板
      try {
        await navigator.clipboard.writeText(code)
        ElMessage.success('代码已复制到剪贴板')
      } catch (clipboardError) {
        // 复制失败，显示悬浮框
        console.log('剪贴板复制失败，显示悬浮框:', clipboardError)
        generatedCode.value = code
        codeTitle.value = `${templateType} 代码`
        showCodeDialog.value = true
      }
    } else {
      ElNotification({
        title: '代码生成失败',
        message: result.error || '生成代码时发生错误',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('代码生成失败:', error)
    ElNotification({
      title: '代码生成失败',
      message: '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right'
    })
  } finally {
    codeGenerationLoading.value = false
  }
}

// 获取代码类型
const getCodeType = (title: string) => {
  if (title.includes('Python') || title.includes('requests') || title.includes('pwntools')) {
    return 'Python'
  } else if (title.includes('curl') || title.includes('bash')) {
    return 'Bash'
  }
  return 'Text'
}

// CodeMirror扩展配置
const codeMirrorExtensions = computed(() => {
  const codeType = getCodeType(codeTitle.value)
  const extensions = []
  
  if (codeType === 'Python') {
    extensions.push(python())
  } else if (codeType === 'Bash') {
    // 对于bash脚本，使用javascript语言包作为近似
    extensions.push(javascript())
  }
  
  // 添加暗色主题
  extensions.push(oneDark)
  
  return extensions
})

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
      selectedData = decodeBase64(flow.b).slice(start, end + 1)
      selectedHex = textToHex(selectedData)
    } else {
      // 复制单个字节
      selectedData = decodeBase64(flow.b)[selection.selectedByte]
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
  let highlighted = text
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

  // 如果有搜索关键字，添加高亮
  if (props.searchKeyword) {
    highlighted = highlightSearchKeyword(highlighted, props.searchKeyword)
  }

  return highlighted
}

// 高亮搜索关键字
const highlightSearchKeyword = (text: string, keyword: string) => {
  if (!keyword) return text

  // 处理正则表达式搜索
  if (keyword.startsWith('/') && keyword.endsWith('/')) {
    const pattern = keyword.slice(1, -1)
    try {
      const regex = new RegExp(pattern, 'gi')
      return text.replace(regex, '<mark class="search-highlight">$&</mark>')
    } catch (error) {
      console.warn('无效的正则表达式:', pattern)
      // 如果正则表达式无效，回退到普通文本搜索
      return highlightText(text, pattern)
    }
  }

  // 普通文本搜索
  return highlightText(text, keyword)
}

// 高亮文本
const highlightText = (text: string, keyword: string) => {
  const escapedKeyword = keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escapedKeyword})`, 'gi')
  return text.replace(regex, '<mark class="search-highlight">$1</mark>')
}

onMounted(() => {
  fetchPcapDetail()
  
  // 添加全局鼠标事件监听
  document.addEventListener('mouseup', endDragSelection)
  document.addEventListener('mouseleave', endDragSelection)
})

// 监听 pcapId 变化，当切换标签页时重新获取数据
watch(() => props.pcapId, (newPcapId, oldPcapId) => {
  if (newPcapId !== oldPcapId) {
    // 重置状态
    pcapDetail.value = null
    flowData.value = []
    flowSize.value = 0
    showSizeWarning.value = false
    showDownloadOption.value = false
    flowSizeInfo.value = null
    
    // 重新获取数据
    fetchPcapDetail()
  }
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
        <div class="section-header">
          <h2 class="section-title">流量基本信息</h2>
          <div class="download-buttons">
            <el-button 
              type="warning" 
              size="small" 
              @click="downloadOriginalFile"
              :disabled="!pcapDetail.filename"
            >
              <el-icon><Download /></el-icon>
              下载原始文件
            </el-button>
            <el-button 
              type="primary" 
              size="small" 
              @click="downloadPcapFile('original')"
              :disabled="!pcapDetail.pcap_file"
            >
              <el-icon><Download /></el-icon>
              下载流量文件
            </el-button>
            <el-button 
              type="success" 
              size="small" 
              @click="downloadPcapFile('parsed')"
              :disabled="!pcapDetail.flow_file"
            >
              <el-icon><Download /></el-icon>
              下载解析文件
            </el-button>
          </div>
        </div>
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
          <div class="info-item info-item-tags" v-if="parseTags(pcapDetail.tags).length > 0">
            <label>标签:</label>
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
              <el-button type="primary" @click="forceLoadFlowData">仍要在线查看</el-button>
            </div>
          </template>
        </el-alert>
      </div>

      <!-- 流量数据 -->
      <div class="flow-section" v-if="flowData.length > 0">
        <div class="section-header">
          <h3 class="section-title">流量数据</h3>
        </div>
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
                <el-button size="small" @click="copyToClipboard(decodeBase64(flow.b))">
                  <el-icon><CopyDocument /></el-icon>
                  复制全部
                </el-button>
                <!-- 代码生成下拉按钮，只在客户端请求时显示 -->
                <el-dropdown 
                  v-if="flow.f === 'c'" 
                  @command="generateCode"
                  :disabled="codeGenerationLoading"
                >
                  <el-button size="small" :loading="codeGenerationLoading">
                    <el-icon><CopyDocument /></el-icon>
                    生成代码
                    <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="requests">
                        <el-icon><CopyDocument /></el-icon>
                        Requests
                      </el-dropdown-item>
                      <el-dropdown-item command="pwntools">
                        <el-icon><CopyDocument /></el-icon>
                        Pwntools
                      </el-dropdown-item>
                      <el-dropdown-item command="curl">
                        <el-icon><CopyDocument /></el-icon>
                        cURL
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
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
                    <div class="text-content" v-html="highlightHTML(decodeBase64(flow.b))"></div>
                  </div>
                  
                  <div v-if="activeTab === 'hex'" class="content-display">
                    <div class="hex-content">{{ textToHex(decodeBase64(flow.b)) }}</div>
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

  <!-- 代码显示对话框 -->
  <el-dialog
    v-model="showCodeDialog"
    :title="codeTitle"
    width="80%"
    :close-on-click-modal="false"
    :close-on-press-escape="true"
    class="code-dialog"
  >
    <div class="code-dialog-content">
      <div class="code-tip">
        <el-icon><InfoFilled /></el-icon>
        自动复制失败，请手动复制以下内容：
      </div>
      <div class="code-display-container">
        <codemirror
          ref="codeMirrorRef"
          v-model="generatedCode"
          :extensions="codeMirrorExtensions"
          :indent-with-tab="true"
          :tab-size="4"
          :read-only="true"
          class="code-display-content"
        />
      </div>
    </div>
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="showCodeDialog = false">关闭</el-button>
        <el-button type="primary" @click="copyToClipboard(generatedCode)">
          <el-icon><CopyDocument /></el-icon>
          复制代码
        </el-button>
      </div>
    </template>
  </el-dialog>
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

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.download-buttons {
  display: flex;
  gap: 8px;
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
  min-height: 40px;
}

.info-item label {
  font-weight: 600;
  color: #606266;
  margin-right: 8px;
  min-width: 80px;
  flex-shrink: 0;
}

.info-item span {
  color: #303133;
  font-family: 'Courier New', monospace;
  flex: 1;
  overflow-x: auto;
  white-space: nowrap;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
  padding: 2px 0;
}

.info-item span::-webkit-scrollbar {
  height: 3px;
}

.info-item span::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 2px;
}

.info-item span::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 2px;
}

.info-item span::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* 标签容器在info-item中的样式 */
.info-item .tags-container {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
  padding: 2px 0;
  white-space: nowrap;
  width: fit-content;
  max-width: 100%;
}

.info-item .tags-container::-webkit-scrollbar {
  height: 3px;
}

.info-item .tags-container::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 2px;
}

.info-item .tags-container::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 2px;
}

.info-item .tags-container::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* 标签项占用两个格子宽度 */
.info-item-tags {
  grid-column: span 2;
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
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
  padding: 4px 0;
}

.tags-container::-webkit-scrollbar {
  height: 3px;
}

.tags-container::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 2px;
}

.tags-container::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 2px;
}

.tags-container::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

.tag-item {
  margin: 0;
  padding: 4px 8px !important;
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

/* 搜索高亮样式 */
.search-highlight {
  background-color: #ffeb3b;
  color: #000;
  padding: 1px 2px;
  border-radius: 2px;
  font-weight: bold;
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

.pcap-download-links {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.download-label {
  font-size: 14px;
  color: #606266;
  font-weight: 500;
}

.download-link {
  font-size: 14px;
}

.flow-download-links {
  display: flex;
  align-items: center;
  gap: 8px;
}

.warning-actions .el-button,
.download-actions .el-button {
  margin: 0;
}

/* 代码对话框样式 */
.code-dialog .el-dialog__header {
  padding: 15px 20px 10px;
}

.code-dialog .el-dialog__body {
  padding: 10px 20px 20px;
}

.code-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.code-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #909399;
  font-size: 14px;
  background: #f4f4f5;
  padding: 10px 15px;
  border-radius: 4px;
  border-left: 4px solid #409eff;
}

.code-display-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}

.code-display-content {
  height: 400px;
  font-size: 14px;
}

.code-display-content .cm-editor {
  height: 100%;
}

.code-display-content .cm-scroller {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

</style>
