<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { ElNotification, ElMessageBox } from 'element-plus'

interface Action {
  id: number
  name: string
  code: string
  output: string
  error: string
  interval: number
  status: string
  next_run: string
}

interface PaginationState {
  currentPage: number
  pageSize: number
  totalItems: number
}

interface SearchState {
  name?: string
  code?: string
  output?: string
}

const props = defineProps<{
  paginationState?: PaginationState
  searchState?: SearchState
}>()

const emit = defineEmits(['edit', 'add', 'state-change'])

const actions = ref<Action[]>([])
const loading = ref(false)

// 分页相关
const currentPage = ref(1)
const pageSize = ref(20)
const totalItems = ref(0)

// 搜索相关
const searchName = ref('')
const searchCode = ref('')
const searchOutput = ref('')

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
    searchName.value = newState.name || ''
    searchCode.value = newState.code || ''
    searchOutput.value = newState.output || ''
  }
}, { immediate: true })

// 监听状态变化并通知父组件
watch([currentPage, pageSize, totalItems], () => {
  // 只有在数据加载完成后才触发状态同步
  if (totalItems.value > 0 || actions.value.length > 0) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        name: searchName.value,
        code: searchCode.value,
        output: searchOutput.value
      }
    })
  }
})

watch([searchName, searchCode, searchOutput], () => {
  // 只有在数据加载完成后才触发状态同步
  if (totalItems.value > 0 || actions.value.length > 0) {
    emit('state-change', {
      pagination: {
        currentPage: currentPage.value,
        pageSize: pageSize.value,
        totalItems: totalItems.value
      },
      search: {
        name: searchName.value,
        code: searchCode.value,
        output: searchOutput.value
      }
    })
  }
})

// 获取Action列表
const fetchActions = async () => {
  loading.value = true
  try {
    const formData = new FormData()
    formData.append('page', currentPage.value.toString())
    formData.append('page_size', pageSize.value.toString())
    if (searchName.value) {
      formData.append('name', searchName.value)
    }
    if (searchCode.value) {
      formData.append('code', searchCode.value)
    }
    if (searchOutput.value) {
      formData.append('output', searchOutput.value)
    }

    const response = await fetch('/webui/action_show', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      actions.value = result.result || []
      totalItems.value = result.total || 0
      
      // 数据加载完成后触发状态同步
      emit('state-change', {
        pagination: {
          currentPage: currentPage.value,
          pageSize: pageSize.value,
          totalItems: totalItems.value
        },
        search: {
          name: searchName.value
        }
      })
    } else {
      ElNotification({
        title: '获取失败',
        message: result.error || '获取Action列表失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('获取Action列表失败:', error)
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

// 删除Action
const deleteAction = async (action: Action) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除定时计划 "${action.name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    // 调用删除API
    const formData = new FormData()
    formData.append('id', action.id.toString())
    
    const response = await fetch('/webui/action_delete', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      ElNotification({
        title: '删除成功',
        message: '定时计划已删除',
        type: 'success',
        position: 'bottom-right'
      })
      // 刷新列表
      fetchActions()
    } else {
      ElNotification({
        title: '删除失败',
        message: result.error || '删除失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除Action失败:', error)
      ElNotification({
        title: '删除失败',
        message: '网络错误，请稍后重试',
        type: 'error',
        position: 'bottom-right'
      })
    }
  }
}

// 编辑Action
const editAction = (action: Action) => {
  emit('edit', action)
}

// 执行Action
const executeAction = async (action: Action) => {
  try {
    const formData = new FormData()
    formData.append('id', action.id.toString())
    
    const response = await fetch('/webui/action_execute', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      ElNotification({
        title: '执行成功',
        message: `定时计划 "${action.name}" 已加入执行队列`,
        type: 'success',
        position: 'bottom-right'
      })
      // 刷新列表
      fetchActions()
    } else {
      ElNotification({
        title: '执行失败',
        message: result.error || '执行失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('执行Action失败:', error)
    ElNotification({
      title: '执行失败',
      message: '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right'
    })
  }
}

// 新增Action
const addAction = () => {
  emit('add')
}

// 搜索
const handleSearch = () => {
  currentPage.value = 1
  fetchActions()
}

// 重置搜索
const resetSearch = () => {
  searchName.value = ''
  searchCode.value = ''
  searchOutput.value = ''
  currentPage.value = 1
  fetchActions()
}

// 分页处理
const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchActions()
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  // 立即触发状态同步
  emit('state-change', {
    pagination: {
      currentPage: currentPage.value,
      pageSize: pageSize.value,
      totalItems: totalItems.value
    },
    search: {
      name: searchName.value
    }
  })
  fetchActions()
}

// 格式化时间
const formatTime = (timeString: string) => {
  if (!timeString) return '-'
  const date = new Date(timeString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 格式化间隔时间
const formatInterval = (interval: number) => {
  if (interval === -1) return '手动执行'
  if (interval === 0) return '不执行'
  if (interval < 60) return `${interval}秒`
  if (interval < 3600) return `${Math.floor(interval / 60)}分钟`
  return `${Math.floor(interval / 3600)}小时`
}

// 格式化状态显示
const formatStatus = (status: string) => {
  const statusMap: Record<string, string> = {
    'PENDING': '等待中',
    'RUNNING': '运行中',
    'SUCCESS': '成功',
    'ERROR': '失败',
    'TIMEOUT': '超时'
  }
  return statusMap[status] || status
}

// 获取状态标签类型
const getStatusType = (status: string) => {
  const typeMap: Record<string, string> = {
    'PENDING': 'info',
    'RUNNING': 'warning',
    'SUCCESS': 'success',
    'ERROR': 'danger',
    'TIMEOUT': 'warning'
  }
  return typeMap[status] || 'info'
}

// 提取代码类型
const getCodeType = (code: string) => {
  if (!code) return '无代码'
  
  // 匹配格式：data:code/python2;base64,xxx 或 data:code/python3;base64,xxx 或 data:code/golang;base64,xxx
  const match = code.match(/^data:code\/(python2|python3|golang);base64,/)
  if (match) {
    const type = match[1]
    switch (type) {
      case 'python2':
        return 'Python 2'
      case 'python3':
        return 'Python 3'
      case 'golang':
        return 'Golang'
      default:
        return '未知类型'
    }
  }
  
  return '格式错误'
}

// 获取代码类型标签颜色
const getCodeTypeTagType = (code: string) => {
  if (!code) return 'info'
  
  const match = code.match(/^data:code\/(python2|python3|golang);base64,/)
  if (match) {
    const type = match[1]
    switch (type) {
      case 'python2':
        return 'warning'
      case 'python3':
        return 'success'
      case 'golang':
        return 'primary'
      default:
        return 'info'
    }
  }
  
  return 'danger'
}

onMounted(() => {
  fetchActions()
})
</script>

<template>
  <div class="action-list-container">
    <!-- 搜索和操作栏 -->
    <div class="toolbar">
      <div class="search-section">
        <el-input
          v-model="searchName"
          placeholder="搜索定时计划名称"
          style="width: 200px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchCode"
          placeholder="搜索代码内容"
          style="width: 200px"
          @keyup.enter="handleSearch"
        />
        <el-input
          v-model="searchOutput"
          placeholder="搜索输出内容"
          style="width: 200px"
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
      
      <div class="action-section">
        <el-button type="primary" @click="addAction">
          <el-icon><Plus /></el-icon>
          新增定时计划
        </el-button>
      </div>
    </div>

    <!-- 表格 -->
    <el-table 
      :data="actions" 
      v-loading="loading"
      stripe
      style="width: 100%"
    >
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column label="执行间隔" width="120">
        <template #default="{ row }">
          <el-tag :type="row.interval === -1 ? 'success' : row.interval === 0 ? 'info' : 'primary'">
            {{ formatInterval(row.interval) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ formatStatus(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="代码类型" width="120">
        <template #default="{ row }">
          <el-tag :type="getCodeTypeTagType(row.code)" size="small">
            {{ getCodeType(row.code) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="output" label="输出" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.output">{{ row.output.substring(0, 100) }}{{ row.output.length > 100 ? '...' : '' }}</span>
          <span v-else class="text-muted">无输出</span>
        </template>
      </el-table-column>
      <el-table-column prop="error" label="错误信息" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.error" class="text-error">{{ row.error.substring(0, 100) }}{{ row.error.length > 100 ? '...' : '' }}</span>
          <span v-else class="text-muted">无错误</span>
        </template>
      </el-table-column>
      <el-table-column label="下次执行" width="160">
        <template #default="{ row }">
          <span v-if="row.interval > 0">{{ formatTime(row.next_run) }}</span>
          <span v-else-if="row.interval === 0" class="text-muted">不执行</span>
          <span v-else class="text-muted">手动执行</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <div class="action-buttons">
            <el-button size="small" type="success" @click="executeAction(row)">
              <el-icon><VideoPlay /></el-icon>
              执行
            </el-button>
            <el-button size="small" @click="editAction(row)">
              <el-icon><Edit /></el-icon>
              编辑
            </el-button>
            <el-button size="small" type="danger" @click="deleteAction(row)">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

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
.action-list-container {
  background: #fff;
  border-radius: 6px;
  border: 1px solid #e6e8eb;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  padding: 20px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 15px;
  border-bottom: 1px solid #e6e8eb;
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

.pagination-container {
  display: flex;
  justify-content: center;
  margin-top: 20px;
  padding-top: 15px;
  border-top: 1px solid #e6e8eb;
}

.text-muted {
  color: #909399;
  font-style: italic;
}

.text-error {
  color: #f56c6c;
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

:deep(.el-table) {
  border-radius: 4px;
}

:deep(.el-table th) {
  background-color: #f5f7fa;
  color: #606266;
  font-weight: 600;
}

:deep(.el-table .el-table__row:hover) {
  background-color: #f5f7fa;
}
</style>
