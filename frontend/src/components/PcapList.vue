<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { ElNotification, ElMessageBox, ElUpload, ElProgress } from 'element-plus'
import { Upload, UploadFilled, Flag } from '@element-plus/icons-vue'

interface PcapItem {
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
  flow_file: string
  flow_data?: string
  pcap_file: string
  tags: string
  size: number
  created_at: string
  updated_at: string
}

interface PaginationState {
  currentPage: number
  pageSize: number
  totalItems: number
}

interface SearchState {
  ip?: string
  tags?: string
  fulltext?: string
  flagInActive?: boolean
  flagOutActive?: boolean
  searchType?: number
}

const props = defineProps<{
  paginationState?: PaginationState
  searchState?: SearchState
}>()

const emit = defineEmits(['view', 'state-change'])

const pcapItems = ref<PcapItem[]>([])
const loading = ref(false)
const uploadRef = ref()

const isRestoringState = ref(false)
const stateRestorationComplete = ref(!props.paginationState && !props.searchState)
const needsStateRestoration = ref(!!(props.paginationState || props.searchState))

const loadCachedData = () => {
  try {
    const cached = localStorage.getItem('pcap-list-cache')
    if (cached) {
      return JSON.parse(cached)
    }
  } catch (error) {
    console.error('加载缓存失败:', error)
  }
  return null
}

const cachedData = ref<{
  items: PcapItem[]
  total: number
  searchKey: string
  timestamp?: number
} | null>(loadCachedData())

const generateStateKey = () => {
  return JSON.stringify({
    ip: searchFilters.value.ip,
    tags: searchFilters.value.tags,
    port: searchFilters.value.port,
    fulltext: searchFilters.value.fulltext,
    flagInActive: flagInActive.value,
    flagOutActive: flagOutActive.value,
    searchType: searchType.value,
    searchMode: searchMode.value,
    currentPage: currentPage.value,
    pageSize: pageSize.value
  })
}

const getCachedData = () => {
  if (!cachedData.value) return null
  
  const stateKey = generateStateKey()
  if (cachedData.value.searchKey === stateKey) {
    return cachedData.value
  }
  return null
}

const getFullStateCachedData = () => {
  if (!cachedData.value) return null
  
  try {
    const currentSearchKey = JSON.stringify({
      ip: searchFilters.value.ip,
      tags: searchFilters.value.tags,
      port: searchFilters.value.port,
      fulltext: searchFilters.value.fulltext,
      flagInActive: flagInActive.value,
      flagOutActive: flagOutActive.value,
      searchType: searchType.value,
      searchMode: searchMode.value
    })
    
    const cachedState = JSON.parse(cachedData.value.searchKey)
    const cachedSearchKey = JSON.stringify({
      ip: cachedState.ip,
      tags: cachedState.tags,
      port: cachedState.port,
      fulltext: cachedState.fulltext,
      flagInActive: cachedState.flagInActive,
      flagOutActive: cachedState.flagOutActive,
      searchType: cachedState.searchType,
      searchMode: cachedState.searchMode
    })
    
    if (currentSearchKey === cachedSearchKey) {
      return cachedData.value
    }
  } catch (error) {
    console.error('解析缓存状态失败:', error)
  }
  return null
}

const setCachedData = (items: PcapItem[], total: number) => {
  const cacheData = {
    items: [...items],
    total,
    searchKey: generateStateKey(),
    timestamp: Date.now()
  }
  cachedData.value = cacheData
  
  try {
    localStorage.setItem('pcap-list-cache', JSON.stringify(cacheData))
  } catch (error) {
    console.error('保存缓存失败:', error)
  }
}

// 缓存信息（用于分页左侧提示）
const cacheInfo = computed(() => {
  const c = cachedData.value
  const ts = (c && c.timestamp) ? (c.timestamp as number) : Date.now()
  const dateStr = new Date(ts).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit'
  })
  const expired = (Date.now() - ts) > 5 * 60 * 1000
  return { dateStr, expired }
})

const uploadDialogVisible = ref(false)
const uploadLoading = ref(false)
const uploadProgress = ref(0)
const uploadStatus = ref('')
const selectedFiles = ref<File[]>([])
const uploadResults = ref<{
  successCount: number
  skippedCount: number
  errorCount: number
  errors: string[]
}>({
  successCount: 0,
  skippedCount: 0,
  errorCount: 0,
  errors: []
})

const currentPage = ref(1)
const pageSize = ref(20)
const totalItems = ref(0)

const searchFilters = ref({
  ip: '',
  tags: '',
  port: '',
  fulltext: ''
})

const searchType = ref(0)
const searchMode = ref('keyword')
const flagInActive = ref(false)
const flagOutActive = ref(false)

watch(() => props.paginationState, (newState) => {
  if (newState) {
    if (!isRestoringState.value) {
      isRestoringState.value = true
      stateRestorationComplete.value = false
    }
    currentPage.value = newState.currentPage
    pageSize.value = newState.pageSize
    totalItems.value = newState.totalItems
  }
}, { immediate: true })

watch(() => props.searchState, (newState) => {
  if (newState) {
    if (!isRestoringState.value) {
      isRestoringState.value = true
      stateRestorationComplete.value = false
    }
    
    if (newState.ip !== undefined && newState.ip !== searchFilters.value.ip) {
      searchFilters.value.ip = newState.ip
    }
    if (newState.tags !== undefined && newState.tags !== searchFilters.value.tags) {
      searchFilters.value.tags = newState.tags
    }
    if (newState.fulltext !== undefined && newState.fulltext !== searchFilters.value.fulltext) {
      searchFilters.value.fulltext = newState.fulltext
    }
    if (newState.flagInActive !== undefined && newState.flagInActive !== flagInActive.value) {
      flagInActive.value = newState.flagInActive
    }
    if (newState.flagOutActive !== undefined && newState.flagOutActive !== flagOutActive.value) {
      flagOutActive.value = newState.flagOutActive
    }
    if (newState.searchType !== undefined && newState.searchType !== searchType.value) {
      searchType.value = newState.searchType
    }
    
    setTimeout(() => {
      isRestoringState.value = false
      stateRestorationComplete.value = true
      
      const cached = getFullStateCachedData()
      if (cached) {
        pcapItems.value = cached.items
        totalItems.value = cached.total
      } else if (isMounted.value && pcapItems.value.length === 0) {
        fetchPcapItems()
      }
    }, 50)
  }
}, { immediate: true, deep: true })

// 只监听 flag 和 searchType 的变化，不监听 searchFilters
watch([flagInActive, flagOutActive, searchType], () => {
  if (isRestoringState.value) {
    return
  }
  
  setTimeout(() => {
    if (!isRestoringState.value && (totalItems.value > 0 || pcapItems.value.length > 0)) {
      currentPage.value = 1
      fetchPcapItems()
    }
  }, 200)
}, { deep: true })

watch([currentPage, pageSize, totalItems], () => {
  if (!isRestoringState.value && (totalItems.value > 0 || pcapItems.value.length > 0)) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        ip: searchFilters.value.ip,
        tags: searchFilters.value.tags,
        fulltext: searchFilters.value.fulltext,
        flagInActive: flagInActive.value,
        flagOutActive: flagOutActive.value,
        searchType: searchType.value
      }
    })
  }
})

watch([searchFilters, flagInActive, flagOutActive, searchType], () => {
  if (!isRestoringState.value && (totalItems.value > 0 || pcapItems.value.length > 0)) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        ip: searchFilters.value.ip,
        tags: searchFilters.value.tags,
        fulltext: searchFilters.value.fulltext,
        flagInActive: flagInActive.value,
        flagOutActive: flagOutActive.value,
        searchType: searchType.value
      }
    })
  }
}, { deep: true })

const fetchPcapItems = async () => {
  if (isRestoringState.value) {
    return
  }

  loading.value = true
  try {
    if ((searchFilters.value.fulltext && searchFilters.value.fulltext.trim() !== '') || searchType.value > 0 || flagInActive.value || flagOutActive.value) {
      await fetchSearchResults()
      return
    }

    const formData = new FormData()
    formData.append('page', currentPage.value.toString())
    formData.append('page_size', pageSize.value.toString())
    
    if (searchFilters.value.ip) {
      formData.append('ip', searchFilters.value.ip)
    }
    if (searchFilters.value.tags) {
      formData.append('tags', searchFilters.value.tags)
    }

    const response = await fetch('/webui/pcap_show', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      pcapItems.value = result.result || []
      totalItems.value = result.total || 0
      
      setCachedData(pcapItems.value, totalItems.value)
      
      emit('state-change', {
        pagination: {
          currentPage: currentPage.value,
          pageSize: pageSize.value,
          totalItems: totalItems.value
        },
        search: {
          ip: searchFilters.value.ip,
          tags: searchFilters.value.tags,
          fulltext: searchFilters.value.fulltext,
          flagInActive: flagInActive.value,
          flagOutActive: flagOutActive.value,
          searchType: searchType.value
        }
      })
    } else {
      ElNotification({
        title: '获取失败',
        message: result.error || '获取流量列表失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('获取流量列表失败:', error)
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

const fetchSearchResults = async () => {
  const cached = getCachedData()
  if (cached && !isRestoringState.value) {
    pcapItems.value = cached.items
    totalItems.value = cached.total
    return
  }

  try {
    const formData = new FormData()
    
    let query = searchFilters.value.fulltext.trim() || '*'
    
    if (flagInActive.value || flagOutActive.value) {
      const flagTags = []
      if (flagInActive.value) flagTags.push('FLAG-IN')
      if (flagOutActive.value) flagTags.push('FLAG-OUT')
      
      if (flagTags.length > 0) {
        if (flagTags.length === 2) {
          query = `tags:FLAG-IN AND tags:FLAG-OUT ${query !== '*' ? 'AND ' + query : ''}`
        } else {
          query = `tags:${flagTags[0]} ${query !== '*' ? 'AND ' + query : ''}`
        }
      }
    }
    
    formData.append('query', query)
    formData.append('page', currentPage.value.toString())
    formData.append('page_size', pageSize.value.toString())
    formData.append('search_type', '0')
    formData.append('search_mode', searchMode.value)
    if (searchFilters.value.port) {
      formData.append('port', searchFilters.value.port)
    }

    const response = await fetch('/webui/search_pcap', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      const searchResults = result.result || []
      pcapItems.value = searchResults.map((item: any) => ({
        id: item.pcap_id,
        src_ip: item.src_ip,
        src_port: item.src_port,
        dst_ip: item.dst_ip,
        dst_port: item.dst_port,
        time: item.timestamp,
        duration: item.duration || 0,
        num_packets: item.num_packets || 0,
        blocked: item.blocked || 'false',
        filename: item.filename || '',
        fingerprints: '',
        suricata: '',
        tags: item.tags,
        size: item.size || 0,
        created_at: '',
        updated_at: '',
        _searchScore: item.score,
        _searchHighlights: item.highlights,
        _searchKeyword: searchFilters.value.fulltext
      }))
      totalItems.value = result.total || 0
      
      setCachedData(pcapItems.value, totalItems.value)
      
      emit('state-change', {
        pagination: {
          currentPage: currentPage.value,
          pageSize: pageSize.value,
          totalItems: totalItems.value
        },
        search: {
          ip: searchFilters.value.ip,
          tags: searchFilters.value.tags,
          fulltext: searchFilters.value.fulltext,
          flagInActive: flagInActive.value,
          flagOutActive: flagOutActive.value,
          searchType: searchType.value
        }
      })
    } else {
      ElNotification({
        title: '搜索失败',
        message: result.error || '搜索失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('搜索失败:', error)
    ElNotification({
      title: '搜索失败',
      message: '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right'
    })
  } finally {
    loading.value = false
  }
}

const viewPcap = (pcapItem: PcapItem) => {
  emit('view', pcapItem)
}

const handleSearch = () => {
  searchType.value = 0
  currentPage.value = 1
  fetchPcapItems()
}

const handleFlagInToggle = () => {
  if (flagInActive.value) {
    flagInActive.value = false
  } else {
    flagInActive.value = true
    flagOutActive.value = false
  }
  updateFlagSearch()
}

const handleFlagOutToggle = () => {
  if (flagOutActive.value) {
    flagOutActive.value = false
  } else {
    flagOutActive.value = true
    flagInActive.value = false
  }
  updateFlagSearch()
}

const updateFlagSearch = () => {
  searchFilters.value.tags = ''
  searchFilters.value.port = ''
  searchFilters.value.fulltext = ''
  searchType.value = 0
  
  currentPage.value = 1
  fetchPcapItems()
}

const resetSearch = () => {
  searchFilters.value = {
    ip: '',
    tags: '',
    port: '',
    fulltext: ''
  }
  searchType.value = 0
  flagInActive.value = false
  flagOutActive.value = false
  currentPage.value = 1
  fetchPcapItems()
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchPcapItems()
  
  emit('state-change', {
    pagination: {
      currentPage: currentPage.value,
      pageSize: pageSize.value,
      totalItems: totalItems.value
    },
    search: {
      ip: searchFilters.value.ip,
      tags: searchFilters.value.tags,
      fulltext: searchFilters.value.fulltext,
      flagInActive: flagInActive.value,
      flagOutActive: flagOutActive.value,
      searchType: searchType.value
    }
  })
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  emit('state-change', {
    pagination: {
      currentPage: currentPage.value,
      pageSize: pageSize.value,
      totalItems: totalItems.value
    },
    search: {
      ip: searchFilters.value.ip,
      tags: searchFilters.value.tags
    }
  })
  fetchPcapItems()
}

const formatTimestamp = (timestamp: number) => {
  const isMilliseconds = timestamp > 1000000000000
  const date = new Date(isMilliseconds ? timestamp : timestamp * 1000)
  
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
  })
}

const parseTags = (tagsStr: string) => {
  try {
    if (!tagsStr || tagsStr === '[]') {
      return []
    }
    
    let normalizedStr = tagsStr
      .replace(/[\u2018\u2019]/g, '"')
      .replace(/[\u201c\u201d]/g, '"')
    
    return JSON.parse(normalizedStr)
  } catch (error) {
    console.warn('解析标签失败:', tagsStr, error)
    return []
  }
}

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
    'STARRED': 'primary',
    // 新增协议解析高亮
    'WEBSOCKET': 'primary',
    'HTTP2': 'info',
    'GRPC': 'success',
    'WS-FRAMES': 'info',
    'GRPC-MSGS': 'success',
    'QUIC': 'warning',
    'HTTP3': 'warning'
  }
  const validTypes = ['primary', 'success', 'warning', 'danger', 'info']
  const key = (tag || '').toUpperCase()
  const type = tagColors[key] || tagColors[tag] || 'info'
  return validTypes.includes(type) ? type : 'info'
}

const formatSize = (size: number) => {
  if (size === 0) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(2)} MB`
  return `${(size / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

const copyToClipboard = async (text: string, type: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElNotification({
      title: '复制成功',
      message: `${type}已复制到剪贴板: ${text}`,
      type: 'success',
      position: 'bottom-right',
      duration: 2000
    })
  } catch (error) {
    ElNotification({
      title: '复制失败',
      message: '无法复制到剪贴板',
      type: 'error',
      position: 'bottom-right',
      duration: 2000
    })
  }
}

const openUploadDialog = () => {
  uploadDialogVisible.value = true
  uploadProgress.value = 0
  uploadStatus.value = ''
  selectedFiles.value = []
  uploadResults.value = {
    successCount: 0,
    skippedCount: 0,
    errorCount: 0,
    errors: []
  }
}

const closeUploadDialog = () => {
  uploadDialogVisible.value = false
  uploadLoading.value = false
  uploadProgress.value = 0
  uploadStatus.value = ''
  selectedFiles.value = []
  uploadResults.value = {
    successCount: 0,
    skippedCount: 0,
    errorCount: 0,
    errors: []
  }
}

const handleBatchUpload = async () => {
  if (selectedFiles.value.length === 0) {
    ElNotification({
      title: '上传失败',
      message: '请选择要上传的文件',
      type: 'warning',
      position: 'bottom-right'
    })
    return
  }

  uploadLoading.value = true
  uploadProgress.value = 0
  uploadStatus.value = `正在上传 ${selectedFiles.value.length} 个文件...`

  const formData = new FormData()
  selectedFiles.value.forEach(file => {
    formData.append('files', file)
  })

  try {
    const progressInterval = setInterval(() => {
      if (uploadProgress.value < 90) {
        uploadProgress.value += Math.random() * 20
      }
    }, 200)

    const response = await fetch('/webui/pcap_upload', {
      method: 'POST',
      body: formData
    })

    clearInterval(progressInterval)
    uploadProgress.value = 100

    const result = await response.json()
    
    if (result.message === 'success' || result.message === 'partial_success') {
      uploadResults.value = {
        successCount: result.success_count || 0,
        skippedCount: result.skipped_count || 0,
        errorCount: result.error_count || 0,
        errors: result.errors || []
      }
      
      uploadProgress.value = 100
      uploadStatus.value = '上传完成'
      
      const totalFiles = selectedFiles.value.length
      const successMsg = `成功上传 ${result.success_count || 0} 个文件`
      const skippedMsg = result.skipped_count > 0 ? `，跳过 ${result.skipped_count} 个重复文件` : ''
      const errorMsg = result.error_count > 0 ? `，失败 ${result.error_count} 个文件` : ''
      
      ElNotification({
        title: '上传完成',
        message: successMsg + skippedMsg + errorMsg,
        type: result.error_count > 0 ? 'warning' : 'success',
        position: 'bottom-right',
        duration: 5000
      })
      
      selectedFiles.value = []
      if (uploadRef.value && uploadRef.value.clearFiles) {
        uploadRef.value.clearFiles()
      }
      
      setTimeout(() => {
        closeUploadDialog()
      }, 2000)
      
      fetchPcapItems()
    } else {
      throw new Error(result.error || '上传失败')
    }
  } catch (error) {
    console.error('批量上传失败:', error)
    uploadProgress.value = 0
    uploadStatus.value = '上传失败'
    ElNotification({
      title: '上传失败',
      message: error instanceof Error ? error.message : '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right',
      duration: 5000
    })
  } finally {
    uploadLoading.value = false
  }
}

const beforeUpload = (file: File) => {
  const isValidType = file.name.endsWith('.pcap') || file.name.endsWith('.pcapng')
  if (!isValidType) {
    ElNotification({
      title: '文件类型错误',
      message: '只能上传 .pcap 或 .pcapng 文件',
      type: 'error',
      position: 'bottom-right'
    })
    return false
  }
  
  const isValidSize = file.size <= 100 * 1024 * 1024
  if (!isValidSize) {
    ElNotification({
      title: '文件过大',
      message: '文件大小不能超过 100MB',
      type: 'error',
      position: 'bottom-right'
    })
    return false
  }
  
  return true
}

const handleFileChange = (file: any, fileList: any[]) => {
  selectedFiles.value = fileList
    .filter(f => f.raw && f.status !== 'fail')
    .map(f => f.raw)
}


const isMounted = ref(false)

onMounted(() => {
  isMounted.value = true
  
  if (!needsStateRestoration.value) {
    fetchPcapItems()
  }
})
</script>

<template>
  <div class="pcap-list-container">
    <!-- 搜索和操作栏 -->
    <div class="toolbar">
      <div class="search-section">
        <el-input
          v-model="searchFilters.fulltext"
          placeholder="全文搜索 (支持正则表达式，如: /flag\{.*\}/)"
          style="width: 300px"
          @keyup.enter="handleSearch"
          clearable
        />
        <el-input
          v-model="searchFilters.ip"
          placeholder="IP地址 (源IP或目标IP)"
          style="width: 180px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchFilters.tags"
          placeholder="标签"
          style="width: 120px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchFilters.port"
          placeholder="端口"
          style="width: 100px"
          @keyup.enter="handleSearch"
        />
        <el-select v-model="searchMode" placeholder="搜索模式" style="width: 120px; margin-right: 10px;">
          <el-option label="关键词搜索" value="keyword"></el-option>
          <el-option label="字符串匹配" value="string"></el-option>
        </el-select>
        <el-button @click="handleSearch">
          <el-icon><Search /></el-icon>
          搜索
        </el-button>
        <div class="flag-buttons">
          <el-button 
            @click="handleFlagInToggle" 
            :type="flagInActive ? 'primary' : 'default'"
            :class="{ 'is-active': flagInActive }"
          >
            <el-icon><Flag /></el-icon>
            IN
          </el-button>
          <el-button 
            @click="handleFlagOutToggle" 
            :type="flagOutActive ? 'primary' : 'default'"
            :class="{ 'is-active': flagOutActive }"
          >
            <el-icon><Flag /></el-icon>
            OUT
          </el-button>
        </div>
        <el-button @click="resetSearch">
          <el-icon><Refresh /></el-icon>
          重置
        </el-button>
      </div>
      <div class="action-section">
        <el-button type="primary" @click="openUploadDialog">
          <el-icon><Upload /></el-icon>
          批量上传
        </el-button>
      </div>
    </div>

    <!-- 表格容器 -->
    <div class="table-container">
      <el-table 
        :data="pcapItems" 
        v-loading="loading"
        stripe
        style="width: 100%"
        @row-click="viewPcap"
        row-class-name="pcap-row"
      >
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column label="流量方向" min-width="300" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="flow-direction" :title="`流量方向: ${row.src_ip}:${row.src_port} -> ${row.dst_ip}:${row.dst_port}`">
            <span 
              class="ip-port clickable" 
              @click.stop="copyToClipboard(`${row.src_ip}:${row.src_port}`, '源地址')"
              :title="`点击复制源地址: ${row.src_ip}:${row.src_port}`"
            >
              {{ row.src_ip }}:{{ row.src_port }}
            </span>
            <el-icon class="arrow-icon"><Right /></el-icon>
            <span 
              class="ip-port clickable" 
              @click.stop="copyToClipboard(`${row.dst_ip}:${row.dst_port}`, '目标地址')"
              :title="`点击复制目标地址: ${row.dst_ip}:${row.dst_port}`"
            >
              {{ row.dst_ip }}:{{ row.dst_port }}
            </span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="标签" min-width="150">
        <template #default="{ row }">
          <div class="tags-container">
            <el-tag
              v-for="tag in parseTags(row.tags)"
              :key="tag"
              :type="getTagType(tag)"
              size="small"
              class="tag-item"
            >
              {{ tag }}
            </el-tag>
            <span v-if="parseTags(row.tags).length === 0" class="text-muted">无标签</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="大小" width="100">
        <template #default="{ row }">
          <span>{{ formatSize(row.size) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="数据包数" width="100">
        <template #default="{ row }">
          <span>{{ row.num_packets }}</span>
        </template>
      </el-table-column>
      <el-table-column label="持续时间" width="100">
        <template #default="{ row }">
          <span>{{ row.duration }}ms</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.blocked === 'true' ? 'danger' : 'success'" size="small">
            {{ row.blocked === 'true' ? '已阻止' : '允许' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" min-width="150">
        <template #default="{ row }">
          <span>{{ formatTimestamp(row.time) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <div class="action-buttons">
            <el-button size="small" type="primary" @click.stop="viewPcap(row)">
              <el-icon><View /></el-icon>
              查看
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
    </div>

    <!-- 分页 -->
    <div class="pagination-container">
      <div class="cache-info-inline">
        <span>缓存时间：{{ cacheInfo.dateStr }}</span>
        <el-tag v-if="cacheInfo.expired" type="warning" size="small">
          缓存超过5分钟，请重置或再次搜索
        </el-tag>
      </div>
      <div class="pagination-center">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          size="small"
          :background="true"
          layout="total, sizes, prev, pager, next"
          :total="totalItems"
          :hide-on-single-page="false"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
      <div class="cache-info-ghost">
        <span>缓存时间：{{ cacheInfo.dateStr }}</span>
        <el-tag v-if="cacheInfo.expired" type="warning" size="small">
          缓存超过5分钟，请重置或再次搜索
        </el-tag>
      </div>
    </div>

    <!-- 批量上传对话框 -->
    <el-dialog
      v-model="uploadDialogVisible"
      title="批量上传PCAP文件"
      width="600px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      @close="closeUploadDialog"
    >
      <div class="upload-container">
        <el-upload
          ref="uploadRef"
          class="upload-dragger"
          drag
          multiple
          :auto-upload="false"
          :before-upload="beforeUpload"
          :on-change="handleFileChange"
          accept=".pcap,.pcapng"
        >
          <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
          <div class="el-upload__text">
            将文件拖到此处，或<em>点击上传</em>
          </div>
          <template #tip>
            <div class="el-upload__tip">
              支持 .pcap 和 .pcapng 格式，单个文件不超过 100MB
            </div>
          </template>
        </el-upload>

        <!-- 上传进度 -->
        <div v-if="uploadLoading || uploadStatus" class="upload-progress">
          <div class="progress-status">{{ uploadStatus }}</div>
          <el-progress 
            :percentage="uploadProgress" 
            :format="(percentage) => Math.round(percentage) + '%'"
            :status="uploadProgress === 100 ? 'success' : ''"
          />
        </div>

        <!-- 上传结果 -->
        <div v-if="uploadResults.successCount > 0 || uploadResults.skippedCount > 0 || uploadResults.errorCount > 0" class="upload-results">
          <h4>上传结果：</h4>
          <div class="result-item">
            <el-tag type="success">成功: {{ uploadResults.successCount }}</el-tag>
            <el-tag type="warning" v-if="uploadResults.skippedCount > 0">跳过: {{ uploadResults.skippedCount }}</el-tag>
            <el-tag type="danger" v-if="uploadResults.errorCount > 0">失败: {{ uploadResults.errorCount }}</el-tag>
          </div>
          
          <!-- 错误详情 -->
          <div v-if="uploadResults.errors.length > 0" class="error-details">
            <h5>错误详情：</h5>
            <ul>
              <li v-for="error in uploadResults.errors" :key="error" class="error-item">
                {{ error }}
              </li>
            </ul>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="closeUploadDialog">取消</el-button>
          <el-button 
            type="primary" 
            @click="handleBatchUpload()"
            :loading="uploadLoading"
          >
            开始上传
          </el-button>
        </div>
      </template>
    </el-dialog>

  </div>
</template>

<style scoped>
.pcap-list-container {
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

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 15px;
  border-bottom: 1px solid #e6e8eb;
  flex-shrink: 0;
}

.search-section {
  display: flex;
  align-items: center;
  gap: 10px;
}

.action-section {
  display: flex;
  align-items: center;
  gap: 10px;
}

.flag-buttons {
  display: flex;
  align-items: center;
  gap: 0;
}

.table-container {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.pagination-container {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  margin-top: 20px;
  padding-top: 15px;
  border-top: 1px solid #e6e8eb;
  flex-shrink: 0;
}

.cache-info-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #606266;
}

.pagination-center {
  display: flex;
  justify-content: center;
}

.cache-info-ghost {
  visibility: hidden;
}

.text-muted {
  color: #909399;
  font-style: italic;
}

.action-buttons {
  display: flex;
  gap: 8px;
  justify-content: flex-start;
  align-items: center;
}

.action-buttons .el-button {
  flex: 0 0 auto;
  min-width: 70px;
}

.flow-direction {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: 'Courier New', monospace;
  min-width: 280px;
  overflow-x: auto;
  white-space: nowrap;
  padding: 4px 0;
  scrollbar-width: thin;
  scrollbar-color: #c1c1c1 #f1f1f1;
}

.flow-direction::-webkit-scrollbar {
  height: 4px;
}

.flow-direction::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 2px;
}

.flow-direction::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 2px;
}

.flow-direction::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

.ip-port {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 12px;
  color: #606266;
}

.ip-port.clickable {
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.ip-port.clickable:hover {
  background: #e6f7ff;
  border-color: #409eff;
  color: #409eff;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(64, 158, 255, 0.2);
}

.arrow-icon {
  color: #409eff;
  font-size: 14px;
}

.tags-container {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.tag-item {
  margin: 0;
}

:deep(.el-table) {
  border-radius: 4px;
}

:deep(.el-table th) {
  background-color: #f5f7fa;
  color: #606266;
  font-weight: 600;
}

:deep(.el-table .pcap-row) {
  cursor: pointer;
}

:deep(.el-table .pcap-row:hover) {
  background-color: #f5f7fa;
}

:deep(.el-table .el-table__row:hover) {
  background-color: #f5f7fa;
}

/* 上传相关样式 */
.upload-container {
  padding: 20px 0;
}

.upload-dragger {
  width: 100%;
}

.upload-progress {
  margin-top: 20px;
}

.progress-status {
  margin-bottom: 10px;
  font-size: 14px;
  color: #606266;
}

.upload-results {
  margin-top: 20px;
  padding: 15px;
  background: #f5f7fa;
  border-radius: 4px;
}

.upload-results h4 {
  margin: 0 0 10px 0;
  color: #303133;
  font-size: 14px;
}

.result-item {
  display: flex;
  gap: 10px;
  margin-bottom: 15px;
}

.error-details {
  margin-top: 15px;
}

.error-details h5 {
  margin: 0 0 10px 0;
  color: #f56c6c;
  font-size: 13px;
}

.error-details ul {
  margin: 0;
  padding-left: 20px;
}

.error-item {
  color: #f56c6c;
  font-size: 12px;
  margin-bottom: 5px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* Flag按钮激活状态样式 */
.is-active {
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2) !important;
  transform: scale(1.05);
  transition: all 0.2s ease;
}
</style>
