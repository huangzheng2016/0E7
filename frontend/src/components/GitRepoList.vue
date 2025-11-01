<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElNotification, ElMessageBox, ElInput } from 'element-plus'
import { Refresh, DocumentCopy } from '@element-plus/icons-vue'

interface GitRepo {
  name: string
  url: string
  description: string
}

const repos = ref<GitRepo[]>([])
const loading = ref(false)
const showEditDialog = ref(false)
const editingRepo = ref<GitRepo | null>(null)
const editDescription = ref('')

// 按名称排序的仓库列表
const sortedRepos = computed(() => {
  return [...repos.value].sort((a, b) => a.name.localeCompare(b.name))
})

// 加载仓库列表
const loadRepos = async () => {
  loading.value = true
  try {
    const formData = new FormData()
    const response = await fetch('/webui/git_repo_list', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    if (result.status === 'success') {
      repos.value = result.data || []
    } else {
      ElNotification({
        title: '错误',
        message: result.msg || '加载仓库列表失败',
        type: 'error'
      })
    }
  } catch (error) {
    console.error('加载仓库列表失败:', error)
    ElNotification({
      title: '错误',
      message: '加载仓库列表失败',
      type: 'error'
    })
  } finally {
    loading.value = false
  }
}

// 开始编辑描述（打开对话框）
const startEditDescription = (repo: GitRepo) => {
  editingRepo.value = repo
  editDescription.value = repo.description || ''
  showEditDialog.value = true
}

// 关闭编辑对话框
const closeEditDialog = () => {
  showEditDialog.value = false
  editingRepo.value = null
  editDescription.value = ''
}

// 保存描述
const saveDescription = async () => {
  if (!editingRepo.value) return

  try {
    const formData = new FormData()
    formData.append('name', editingRepo.value.name)
    formData.append('description', editDescription.value)

    const response = await fetch('/webui/git_repo_update_description', {
      method: 'POST',
      body: formData
    })

    const result = await response.json()
    if (result.status === 'success') {
      ElNotification({
        title: '成功',
        message: '描述更新成功',
        type: 'success'
      })
      
      // 更新本地数据
      const repo = repos.value.find(r => r.name === editingRepo.value!.name)
      if (repo) {
        repo.description = editDescription.value
      }
      
      closeEditDialog()
    } else {
      ElNotification({
        title: '错误',
        message: result.msg || '更新描述失败',
        type: 'error'
      })
    }
  } catch (error) {
    console.error('更新描述失败:', error)
    ElNotification({
      title: '错误',
      message: '更新描述失败',
      type: 'error'
    })
  }
}

// 删除仓库
const deleteRepo = async (repo: GitRepo) => {
  try {
    const { value } = await ElMessageBox.prompt(
      `<div style="text-align: left; line-height: 1.8;">
        <p style="margin-bottom: 8px;">您即将删除仓库: <strong style="color: #409eff;">${repo.name}</strong></p>
        <p style="margin-bottom: 12px; color: #909399;">此操作不可恢复，请仔细确认！</p>
        <p style="margin: 0;">请输入仓库名称 <code style="background: #f5f7fa; padding: 2px 6px; border-radius: 3px;">${repo.name}</code> 以确认删除：</p>
      </div>`,
      '删除仓库确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        dangerouslyUseHTMLString: true,
        inputPlaceholder: `请输入 ${repo.name}`,
        inputPattern: new RegExp(`^${repo.name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`),
        inputErrorMessage: `请输入正确的仓库名称: ${repo.name}`,
        inputValidator: (value: string) => {
          if (!value || value.trim() === '') {
            return '请输入仓库名称'
          }
          if (value !== repo.name) {
            return `仓库名称不匹配，请输入: ${repo.name}`
          }
          return true
        },
        customClass: 'delete-repo-dialog'
      }
    )

    // 验证输入（虽然已经有 inputPattern 和 inputValidator，但再次确认）
    if (!value || value.trim() !== repo.name) {
      ElNotification({
        title: '错误',
        message: '输入的仓库名称不匹配，删除已取消',
        type: 'error',
        duration: 3000
      })
      return
    }

    const formData = new FormData()
    formData.append('name', repo.name)

    const response = await fetch('/webui/git_repo_delete', {
      method: 'POST',
      body: formData
    })

    const result = await response.json()
    if (result.status === 'success') {
      ElNotification({
        title: '成功',
        message: '仓库删除成功',
        type: 'success'
      })
      
      // 从列表中移除
      repos.value = repos.value.filter(r => r.name !== repo.name)
    } else {
      ElNotification({
        title: '错误',
        message: result.msg || '删除仓库失败',
        type: 'error'
      })
    }
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('删除仓库失败:', error)
      ElNotification({
        title: '错误',
        message: '删除仓库失败',
        type: 'error'
      })
    }
  }
}

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

onMounted(() => {
  loadRepos()
})
</script>

<template>
  <div class="git-repo-list">
    <div class="header">
      <h2>Git 仓库管理</h2>
      <el-button 
        type="primary" 
        @click="loadRepos"
        :loading="loading"
      >
        <el-icon v-if="!loading"><Refresh /></el-icon>
        {{ loading ? '加载中...' : '刷新' }}
      </el-button>
    </div>

    <div class="repo-table-container" v-loading="loading">
      <el-table :data="sortedRepos" stripe style="width: 100%">
        <el-table-column prop="name" label="仓库名称" width="200">
          <template #default="{ row }">
            <strong>{{ row.name }}</strong>
          </template>
        </el-table-column>

        <el-table-column prop="url" label="GIT URL" min-width="300">
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

        <el-table-column prop="description" label="描述" min-width="300">
          <template #default="{ row }">
            <div class="description-cell">
              <span v-if="row.description">{{ row.description }}</span>
              <span v-else class="empty-description">暂无描述</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-button
                type="primary"
                size="small"
                @click="startEditDescription(row)"
              >
                修改描述
              </el-button>
              <el-button
                type="danger"
                size="small"
                @click="deleteRepo(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="sortedRepos.length === 0 && !loading" class="empty-state">
        <p>暂无仓库</p>
        <p class="hint">仓库会在首次 push 时自动创建</p>
      </div>
    </div>

    <!-- 编辑描述对话框 -->
    <el-dialog
      v-model="showEditDialog"
      :title="`编辑仓库描述 - ${editingRepo?.name || ''}`"
      width="500px"
      @close="closeEditDialog"
    >
      <el-form label-width="80px">
        <el-form-item label="仓库名称">
          <el-input :value="editingRepo?.name" disabled />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="editDescription"
            type="textarea"
            :rows="4"
            placeholder="请输入仓库描述"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="closeEditDialog">取消</el-button>
          <el-button type="primary" @click="saveDescription">
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.git-repo-list {
  padding: 20px;
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header h2 {
  margin: 0;
  font-size: 20px;
  color: #303133;
}

.repo-table-container {
  min-height: 200px;
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

.description-cell {
  color: #606266;
}

.empty-description {
  color: #909399;
  font-style: italic;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: #909399;
}

.empty-state .hint {
  font-size: 12px;
  color: #c0c4cc;
  margin-top: 8px;
}

/* 删除确认对话框样式 */
:deep(.delete-repo-dialog) {
  .el-message-box__message {
    p {
      margin: 0;
    }
    code {
      font-family: 'Courier New', monospace;
      font-size: 13px;
    }
  }
  .el-message-box__input {
    .el-input__inner {
      font-family: 'Courier New', monospace;
    }
  }
  .el-button--warning {
    background-color: #f56c6c;
    border-color: #f56c6c;
  }
  .el-button--warning:hover {
    background-color: #f78989;
    border-color: #f78989;
  }
}
</style>

