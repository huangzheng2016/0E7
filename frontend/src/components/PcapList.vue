<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { ElNotification, ElMessageBox } from 'element-plus'

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
  fingerprints: string
  suricata: string
  flow: string
  tags: string
  size: string
  created_at: string
  updated_at: string
}

interface PaginationState {
  currentPage: number
  pageSize: number
  totalItems: number
}

interface SearchState {
  src_ip?: string
  dst_ip?: string
  tags?: string
}

const props = defineProps<{
  paginationState?: PaginationState
  searchState?: SearchState
}>()

const emit = defineEmits(['view', 'state-change'])

const pcapItems = ref<PcapItem[]>([])
const loading = ref(false)

// 分页相关
const currentPage = ref(1)
const pageSize = ref(20)
const totalItems = ref(0)

// 搜索相关
const searchFilters = ref({
  src_ip: '',
  dst_ip: '',
  tags: ''
})

// 状态同步
watch(() => props.paginationState, (newState) => {
  if (newState) {
    currentPage.value = newState.currentPage
    pageSize.value = newState.pageSize
    totalItems.value = newState.totalItems
  }
}, { immediate: true })

watch(() => props.searchState, (newState) => {
  if (newState) {
    searchFilters.value.src_ip = newState.src_ip || ''
    searchFilters.value.dst_ip = newState.dst_ip || ''
    searchFilters.value.tags = newState.tags || ''
  }
}, { immediate: true })

// 监听状态变化并通知父组件
watch([currentPage, pageSize, totalItems], () => {
  if (totalItems.value > 0 || pcapItems.value.length > 0) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        src_ip: searchFilters.value.src_ip,
        dst_ip: searchFilters.value.dst_ip,
        tags: searchFilters.value.tags
      }
    })
  }
})

watch(searchFilters, () => {
  if (totalItems.value > 0 || pcapItems.value.length > 0) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        src_ip: searchFilters.value.src_ip,
        dst_ip: searchFilters.value.dst_ip,
        tags: searchFilters.value.tags
      }
    })
  }
}, { deep: true })

// 获取流量列表
const fetchPcapItems = async () => {
  loading.value = true
  try {
    const formData = new FormData()
    formData.append('page', currentPage.value.toString())
    formData.append('page_size', pageSize.value.toString())
    
    // 添加搜索过滤器
    if (searchFilters.value.src_ip) {
      formData.append('src_ip', searchFilters.value.src_ip)
    }
    if (searchFilters.value.dst_ip) {
      formData.append('dst_ip', searchFilters.value.dst_ip)
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
      
      // 数据加载完成后触发状态同步
      emit('state-change', {
        pagination: {
          currentPage: currentPage.value,
          pageSize: pageSize.value,
          totalItems: totalItems.value
        },
        search: {
          src_ip: searchFilters.value.src_ip,
          dst_ip: searchFilters.value.dst_ip,
          tags: searchFilters.value.tags
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

// 查看流量详情
const viewPcap = (pcapItem: PcapItem) => {
  emit('view', pcapItem)
}

// 搜索
const handleSearch = () => {
  currentPage.value = 1
  fetchPcapItems()
}

// 重置搜索
const resetSearch = () => {
  searchFilters.value = {
    src_ip: '',
    dst_ip: '',
    tags: ''
  }
  currentPage.value = 1
  fetchPcapItems()
}

// 分页处理
const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchPcapItems()
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
      src_ip: searchFilters.value.src_ip,
      dst_ip: searchFilters.value.dst_ip,
      tags: searchFilters.value.tags
    }
  })
  fetchPcapItems()
}

// 格式化时间戳
const formatTimestamp = (timestamp: number) => {
  // 判断时间戳是秒级还是毫秒级
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
  })
}

// 解析标签
const parseTags = (tagsStr: string) => {
  try {
    return JSON.parse(tagsStr || '[]')
  } catch {
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

onMounted(() => {
  fetchPcapItems()
})
</script>

<template>
  <div class="pcap-list-container">
    <!-- 搜索和操作栏 -->
    <div class="toolbar">
      <div class="search-section">
        <el-input
          v-model="searchFilters.src_ip"
          placeholder="源IP"
          style="width: 150px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchFilters.dst_ip"
          placeholder="目标IP"
          style="width: 150px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchFilters.tags"
          placeholder="标签"
          style="width: 150px"
          @keyup.enter="handleSearch"
        />
        <el-button @click="handleSearch">
          <el-icon><Search /></el-icon>
          搜索
        </el-button>
        <el-button @click="resetSearch">
          <el-icon><Refresh /></el-icon>
          重置
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
      <el-table-column label="流量方向" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="flow-direction">
            <span class="ip-port">{{ row.src_ip }}:{{ row.src_port }}</span>
            <el-icon class="arrow-icon"><Right /></el-icon>
            <span class="ip-port">{{ row.dst_ip }}:{{ row.dst_port }}</span>
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
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :small="true"
        :background="true"
        layout="total, sizes, prev, pager, next"
        :total="totalItems"
        :hide-on-single-page="false"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>

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

.table-container {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.pagination-container {
  display: flex;
  justify-content: center;
  margin-top: 20px;
  padding-top: 15px;
  border-top: 1px solid #e6e8eb;
  flex-shrink: 0;
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
}

.ip-port {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 12px;
  color: #606266;
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
</style>
