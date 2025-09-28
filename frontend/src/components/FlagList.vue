<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search, Refresh, Edit, Delete } from '@element-plus/icons-vue'

interface Flag {
  id: number
  exploit_id: number
  team: string
  flag: string
  status: string
  msg: string
  created_at: string
  updated_at: string
  exploit_name?: string
}

interface FlagSubmitDialog {
  visible: boolean
  flag: string
  team: string
  flagRegex: string
  loading: boolean
}

const flags = ref<Flag[]>([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

// 自动刷新相关
const autoRefresh = ref(false)
const refreshInterval = ref(5000) // 5秒刷新一次
let refreshTimer: number | null = null

// 搜索条件
const searchForm = reactive({
  flag: '',
  team: '',
  status: '',
  exploit_id: ''
})

// 手动提交flag弹窗
const submitDialog = reactive<FlagSubmitDialog>({
  visible: false,
  flag: '',
  team: '',
  flagRegex: '',
  loading: false
})

// 状态选项
const statusOptions = [
  { label: '全部', value: '' },
  { label: '队列中', value: 'QUEUE' },
  { label: '成功', value: 'SUCCESS' },
  { label: '失败', value: 'FAILED' },
  { label: '跳过', value: 'SKIPPED' }
]

// 状态标签样式
const getStatusTagType = (status: string) => {
  switch (status) {
    case 'SUCCESS':
      return 'success'
    case 'FAILED':
      return 'danger'
    case 'SKIPPED':
      return 'warning'
    case 'QUEUE':
      return 'info'
    default:
      return ''
  }
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'SUCCESS':
      return '成功'
    case 'FAILED':
      return '失败'
    case 'SKIPPED':
      return '跳过'
    case 'QUEUE':
      return '队列中'
    default:
      return status
  }
}

// 获取flag列表
const fetchFlags = async () => {
  loading.value = true
  try {
    const formData = new FormData()
    formData.append('page', currentPage.value.toString())
    formData.append('page_size', pageSize.value.toString())
    
    if (searchForm.flag) {
      formData.append('flag', searchForm.flag)
    }
    if (searchForm.team) {
      formData.append('team', searchForm.team)
    }
    if (searchForm.status) {
      formData.append('status', searchForm.status)
    }
    if (searchForm.exploit_id) {
      formData.append('exploit_id', searchForm.exploit_id)
    }

    const response = await fetch('/webui/flag_show', {
      method: 'POST',
      body: formData
    })
    const data = await response.json()
    
    if (data.message === 'success') {
      flags.value = data.result.flags || []
      total.value = data.result.total || 0
    } else {
      ElMessage.error(data.error || '获取flag列表失败')
    }
  } catch (error) {
    console.error('获取flag列表失败:', error)
    ElMessage.error('获取flag列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  currentPage.value = 1
  fetchFlags()
  // 搜索后重新开始自动刷新
  if (autoRefresh.value) {
    startAutoRefresh()
  }
}

// 重置搜索
const handleReset = () => {
  searchForm.flag = ''
  searchForm.team = ''
  searchForm.status = ''
  searchForm.exploit_id = ''
  currentPage.value = 1
  fetchFlags()
  // 重置后重新开始自动刷新
  if (autoRefresh.value) {
    startAutoRefresh()
  }
}

// 刷新
const handleRefresh = () => {
  fetchFlags()
}

// 开始自动刷新
const startAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (autoRefresh.value) {
    refreshTimer = setInterval(() => {
      fetchFlags()
    }, refreshInterval.value)
  }
}

// 停止自动刷新
const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

// 切换自动刷新状态
const toggleAutoRefresh = () => {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

// 分页变化
const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchFlags()
  // 分页后重新开始自动刷新
  if (autoRefresh.value) {
    startAutoRefresh()
  }
}

const handlePageSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchFlags()
  // 分页大小变化后重新开始自动刷新
  if (autoRefresh.value) {
    startAutoRefresh()
  }
}

// 打开手动提交弹窗
const openSubmitDialog = () => {
  submitDialog.visible = true
  submitDialog.flag = ''
  submitDialog.team = ''
  submitDialog.flagRegex = ''
}

// 关闭手动提交弹窗
const closeSubmitDialog = () => {
  submitDialog.visible = false
  submitDialog.flag = ''
  submitDialog.team = ''
  submitDialog.flagRegex = ''
}

// 手动提交flag
const handleSubmitFlag = async () => {
  if (!submitDialog.flag.trim()) {
    ElMessage.warning('请输入flag')
    return
  }

  submitDialog.loading = true
  try {
    const formData = new FormData()
    formData.append('flag', submitDialog.flag.trim())
    if (submitDialog.team.trim()) {
      formData.append('team', submitDialog.team.trim())
    }
    if (submitDialog.flagRegex.trim()) {
      formData.append('flag_regex', submitDialog.flagRegex.trim())
    }

    const response = await fetch('/webui/flag/submit', {
      method: 'POST',
      body: formData
    })
    
    const data = await response.json()
    
    if (data.message === 'success') {
      const result = data.result
      let message = `提交完成！总计: ${result.total} 条`
      if (result.success > 0) {
        message += `，成功: ${result.success} 条`
      }
      if (result.skipped > 0) {
        message += `，跳过: ${result.skipped} 条`
      }
      if (result.error > 0) {
        message += `，失败: ${result.error} 条`
      }
      
      ElMessage.success(message)
      closeSubmitDialog()
      fetchFlags()
    } else {
      ElMessage.error(data.error || 'Flag提交失败')
    }
  } catch (error) {
    console.error('提交flag失败:', error)
    ElMessage.error('提交flag失败')
  } finally {
    submitDialog.loading = false
  }
}

// 删除flag
const handleDelete = async (flag: Flag) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除flag "${flag.flag}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    const formData = new FormData()
    formData.append('id', flag.id.toString())

    const response = await fetch('/webui/flag_delete', {
      method: 'POST',
      body: formData
    })
    
    const data = await response.json()
    
    if (data.message === 'success') {
      ElMessage.success('删除成功')
      fetchFlags()
    } else {
      ElMessage.error(data.error || '删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除flag失败:', error)
      ElMessage.error('删除失败')
    }
  }
}

// 格式化时间
const formatTime = (timeStr: string) => {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN')
}

// 监听搜索条件变化
watch([() => searchForm.flag, () => searchForm.team, () => searchForm.status, () => searchForm.exploit_id], () => {
  // 可以添加防抖逻辑
}, { deep: true })

onMounted(() => {
  fetchFlags()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div class="flag-list">
    <!-- 搜索区域 -->
    <div class="search-section">
      <el-card class="search-card">
        <div class="search-form">
          <el-form :model="searchForm" inline>
            <el-form-item label="Flag">
              <el-input
                v-model="searchForm.flag"
                placeholder="请输入flag"
                clearable
                style="width: 200px"
                @keyup.enter="handleSearch"
              />
            </el-form-item>
            <el-form-item label="Team">
              <el-input
                v-model="searchForm.team"
                placeholder="请输入team"
                clearable
                style="width: 200px"
                @keyup.enter="handleSearch"
              />
            </el-form-item>
            <el-form-item label="状态">
              <el-select
                v-model="searchForm.status"
                placeholder="请选择状态"
                clearable
                style="width: 150px"
              >
                <el-option
                  v-for="option in statusOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="Exploit ID">
              <el-input
                v-model="searchForm.exploit_id"
                placeholder="请输入exploit ID"
                clearable
                style="width: 150px"
                @keyup.enter="handleSearch"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :icon="Search" @click="handleSearch">
                搜索
              </el-button>
              <el-button :icon="Refresh" @click="handleReset">
                重置
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-card>
    </div>

    <!-- 操作区域 -->
    <div class="action-section">
      <el-card>
        <div class="action-bar">
          <div class="action-left">
            <el-button type="primary" :icon="Plus" @click="openSubmitDialog">
              手动提交Flag
            </el-button>
            <el-button :icon="Refresh" @click="handleRefresh">
              刷新
            </el-button>
            <el-button 
              :type="autoRefresh ? 'success' : 'default'" 
              @click="toggleAutoRefresh"
            >
              {{ autoRefresh ? '停止自动刷新' : '开启自动刷新' }}
            </el-button>
          </div>
          <div class="action-right">
            <span class="total-info">共 {{ total }} 条记录</span>
            <span v-if="autoRefresh" class="auto-refresh-info">
              ({{ refreshInterval / 1000 }}秒自动刷新)
            </span>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 数据表格 -->
    <div class="table-section">
      <el-card>
        <el-table
          :data="flags"
          v-loading="loading"
          stripe
          style="width: 100%"
          height="calc(100vh - 300px)"
        >
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="flag" label="Flag" min-width="200">
            <template #default="{ row }">
              <el-text class="flag-text" type="primary">{{ row.flag }}</el-text>
            </template>
          </el-table-column>
          <el-table-column prop="team" label="Team" width="120" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusTagType(row.status)">
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="msg" label="消息" min-width="150">
            <template #default="{ row }">
              <el-text v-if="row.msg" type="info">{{ row.msg }}</el-text>
              <el-text v-else type="info">-</el-text>
            </template>
          </el-table-column>
          <el-table-column prop="exploit_id" label="Exploit ID" width="100" />
          <el-table-column prop="exploit_name" label="Exploit名称" width="150">
            <template #default="{ row }">
              <el-text v-if="row.exploit_name" type="primary">{{ row.exploit_name }}</el-text>
              <el-text v-else type="info">手动提交</el-text>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="180">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column prop="updated_at" label="更新时间" width="180">
            <template #default="{ row }">
              {{ formatTime(row.updated_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button
                type="danger"
                size="small"
                :icon="Delete"
                @click="handleDelete(row)"
              >
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <!-- 分页 -->
        <div class="pagination-section">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handlePageSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </el-card>
    </div>

    <!-- 手动提交Flag弹窗 -->
    <el-dialog
      v-model="submitDialog.visible"
      title="手动提交Flag"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form :model="submitDialog" label-width="100px">
        <el-form-item label="Flag" required>
          <el-input
            v-model="submitDialog.flag"
            placeholder="请输入flag，支持批量提交：&#10;1. 每行一个flag&#10;2. 逗号分隔多个flag&#10;3. 最多999条"
            type="textarea"
            :rows="8"
          />
          <div class="form-tip">
            <el-text type="info" size="small">
              支持批量提交：每行一个flag，或用逗号分隔，最多999条
            </el-text>
          </div>
        </el-form-item>
        <el-form-item label="Team">
          <el-input
            v-model="submitDialog.team"
            placeholder="请输入team（可选）"
          />
        </el-form-item>
        <el-form-item label="Flag正则">
          <el-input
            v-model="submitDialog.flagRegex"
            placeholder="请输入flag正则表达式（可选，不填则使用服务器默认）"
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="closeSubmitDialog">取消</el-button>
          <el-button
            type="primary"
            :loading="submitDialog.loading"
            @click="handleSubmitFlag"
          >
            提交
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.flag-list {
  padding: 0;
}

.search-section {
  margin-bottom: 16px;
}

.search-card {
  border-radius: 8px;
}

.search-form {
  padding: 0;
}

.action-section {
  margin-bottom: 16px;
}

.action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.action-left {
  display: flex;
  gap: 12px;
}

.action-right {
  display: flex;
  align-items: center;
}

.total-info {
  color: #606266;
  font-size: 14px;
}

.table-section {
  margin-bottom: 16px;
}

.flag-text {
  font-family: 'Courier New', monospace;
  font-size: 13px;
  word-break: break-all;
}

.pagination-section {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.form-tip {
  margin-top: 8px;
}

.auto-refresh-info {
  margin-left: 12px;
  color: #67c23a;
  font-size: 12px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .search-form .el-form {
    flex-direction: column;
    align-items: stretch;
  }
  
  .search-form .el-form-item {
    margin-right: 0;
    margin-bottom: 12px;
  }
  
  .action-bar {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
  
  .action-left {
    justify-content: center;
  }
  
  .action-right {
    justify-content: center;
  }
}
</style>
