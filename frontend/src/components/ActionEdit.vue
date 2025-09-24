<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElNotification } from 'element-plus'
import CodeEditor from './CodeEditor.vue'

interface Action {
  id: number
  name: string
  code: string
  output: string
  interval: number
  updated: string
}

const props = defineProps<{
  modelValue?: boolean
  action: Action | null
  isEditing: boolean
  standalone?: boolean
}>()

const emit = defineEmits(['update:modelValue', 'save-success'])

const dialogVisible = computed({
  get: () => props.modelValue ?? true,
  set: (value) => emit('update:modelValue', value)
})

// 如果standalone为true，则显示独立模式
const isStandalone = computed(() => props.standalone === true)

const form = ref({
  id: 0,
  name: '',
  code: '',
  output: '',
  interval: 0,
  code_language: 'python3'
})

const loading = ref(false)

// 监听action变化，更新表单
watch(() => props.action, (newAction) => {
  if (newAction) {
    form.value = {
      id: newAction.id,
      name: newAction.name,
      code: newAction.code,
      output: newAction.output,
      interval: newAction.interval,
      code_language: 'python3'
    }
    
    // 解析代码语言
    if (newAction.code && newAction.code.startsWith('data:code/')) {
      try {
        const parts = newAction.code.split(';base64,')
        if (parts.length === 2) {
          const langPart = parts[0].split('/')
          if (langPart.length === 2) {
            form.value.code_language = langPart[1]
          }
          
          // 解码base64
          const base64Code = parts[1]
          const decodedCode = atob(base64Code)
          form.value.code = decodedCode
        }
      } catch (error) {
        console.error('Base64解码失败:', error)
        form.value.code = newAction.code
      }
    }
  }
}, { immediate: true })

// 编码代码为base64格式
const encodeCodeToBase64 = (code: string, language: string): string => {
  if (!code.trim()) return ''
  const base64Code = btoa(unescape(encodeURIComponent(code)))
  return `data:code/${language};base64,${base64Code}`
}

// 保存Action
const saveAction = async () => {
  if (!form.value.name.trim()) {
    ElNotification({
      title: '验证失败',
      message: '请输入定时计划名称',
      type: 'error',
      position: 'bottom-right'
    })
    return
  }

  loading.value = true
  try {
    const formData = new FormData()
    
    if (props.isEditing && form.value.id > 0) {
      formData.append('id', form.value.id.toString())
    }
    
    formData.append('name', form.value.name)
    formData.append('output', form.value.output)
    formData.append('interval', form.value.interval.toString())
    
    // 处理代码
    if (form.value.code.trim()) {
      const encodedCode = encodeCodeToBase64(form.value.code, form.value.code_language)
      formData.append('code', encodedCode)
    } else {
      formData.append('code', '')
    }

    const response = await fetch('/webui/action', {
      method: 'POST',
      body: formData
    })
    
    const result = await response.json()
    
    if (result.message === 'success') {
      ElNotification({
        title: '保存成功',
        message: props.isEditing ? '定时计划更新成功' : '定时计划创建成功',
        type: 'success',
        position: 'bottom-right'
      })
      emit('save-success')
    } else {
      ElNotification({
        title: '保存失败',
        message: result.error || '保存失败',
        type: 'error',
        position: 'bottom-right'
      })
    }
  } catch (error) {
    console.error('保存Action失败:', error)
    ElNotification({
      title: '保存失败',
      message: '网络错误，请稍后重试',
      type: 'error',
      position: 'bottom-right'
    })
  } finally {
    loading.value = false
  }
}

// 取消编辑
const cancelEdit = () => {
  dialogVisible.value = false
}

// 重置表单
const resetForm = () => {
  form.value = {
    id: 0,
    name: '',
    code: '',
    output: '',
    interval: 0,
    code_language: 'python3'
  }
}
</script>

<template>
  <div v-if="isStandalone" class="standalone-edit">
      <div class="edit-header">
        <h2>{{ isEditing ? `${form.name || '未命名'} - 编辑定时计划` : '新增定时计划' }}</h2>
      </div>
    <div class="edit-content">
      <el-form :model="form" label-width="120px" :rules="{}">
        <div class="form-section">
          <h3>基本信息</h3>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
              <el-form-item label="名称" required>
                <el-input
                  v-model="form.name"
                  placeholder="请输入定时计划名称"
                  maxlength="255"
                  show-word-limit
                />
              </el-form-item>
            </el-col>
            
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
              <el-form-item label="执行间隔">
                <el-input-number
                  v-model="form.interval"
                  :min="-1"
                  :max="86400"
                  placeholder="执行间隔（秒）"
                  style="width: 100%"
                />
                <div class="form-tip">
                  -1: 手动执行, 0: 不执行, >0: 间隔秒数
                </div>
              </el-form-item>
            </el-col>
          </el-row>
        </div>
        
        <div class="form-section">
          <h3>代码内容</h3>
          <el-form-item label-width="0">
            <CodeEditor
              v-model="form.code"
              :language="form.code_language"
              @update:language="(lang) => form.code_language = lang"
              placeholder="请输入要执行的代码..."
            />
          </el-form-item>
          
          <h3>输出</h3>
          <el-form-item label-width="0">
            <el-input
              v-model="form.output"
              type="textarea"
              :rows="4"
              placeholder="输出内容（可选）"
              maxlength="10000"
              show-word-limit
            />
          </el-form-item>
        </div>
      </el-form>
      
      <div class="edit-footer">
        <el-button @click="cancelEdit">取消</el-button>
        <el-button type="primary" @click="saveAction" :loading="loading">
          {{ isEditing ? '更新' : '创建' }}
        </el-button>
      </div>
    </div>
  </div>
  
  <el-dialog
    v-if="!isStandalone"
    v-model="dialogVisible"
    :title="isEditing ? '编辑定时计划' : '新增定时计划'"
    width="80%"
    :close-on-click-modal="false"
    @close="resetForm"
  >
    <el-form :model="form" label-width="100px" :rules="{}">
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="名称" required>
            <el-input
              v-model="form.name"
              placeholder="请输入定时计划名称"
              maxlength="255"
              show-word-limit
            />
          </el-form-item>
        </el-col>
        
      </el-row>
      
      <h3>输出</h3>
      <el-form-item>
        <el-input
          v-model="form.output"
          type="textarea"
          :rows="4"
          placeholder="输出内容（可选）"
          maxlength="10000"
          show-word-limit
        />
      </el-form-item>
    </el-form>
    
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="cancelEdit">取消</el-button>
        <el-button type="primary" @click="saveAction" :loading="loading">
          {{ isEditing ? '更新' : '创建' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:deep(.el-dialog__body) {
  padding: 20px;
}

:deep(.el-form-item) {
  margin-bottom: 20px;
}

:deep(.el-form-item__label) {
  font-weight: 600;
  color: #606266;
}

/* 独立编辑样式 */
.standalone-edit {
  background: #fff;
  border-radius: 6px;
  border: 1px solid #e6e8eb;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  overflow: hidden;
  width: 100%;
}

.edit-header {
  background: #f5f7fa;
  padding: 20px;
  border-bottom: 1px solid #e6e8eb;
}

.edit-header h2 {
  margin: 0;
  color: #303133;
  font-size: 18px;
  font-weight: 600;
}

.edit-content {
  padding: 20px;
  width: 100%;
}

.edit-content .el-form {
  width: 100%;
}

/* 代码编辑器响应式样式 */
.edit-content .code-editor-container {
  width: 100%;
  min-height: 400px;
  display: flex;
  flex-direction: column;
}

.edit-content .code-editor-container .cm-editor {
  flex: 1;
  min-height: 400px;
  width: 100%;
}

.form-section {
  margin-bottom: 30px;
}

.form-section h3 {
  margin: 0 0 15px 0;
  padding: 10px 0;
  border-bottom: 1px solid #e6e8eb;
  color: #303133;
  font-size: 16px;
  font-weight: 600;
}

.form-section:last-child {
  margin-bottom: 0;
}

.edit-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 20px;
  padding: 20px;
  border-top: 1px solid #e6e8eb;
  background: #fafafa;
}
</style>
