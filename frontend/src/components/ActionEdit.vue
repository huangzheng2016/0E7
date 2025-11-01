<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { ElNotification } from 'element-plus'
import CodeEditor from './CodeEditor.vue'

interface Action {
  id: number
  name: string
  code: string
  output: string
  error: string
  interval: number
  timeout: number
  next_run: string
  config?: string
}

const props = defineProps<{
  modelValue?: boolean
  action?: Action | null
  actionId?: number | string
  isEditing: boolean
  standalone?: boolean
}>()

const emit = defineEmits(['update:modelValue', 'save-success', 'close'])

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
  error: '',
  interval: 0,
  timeout: 30,
  code_language: 'python3',
  next_run: '',
  config_type: '',
  config_num: 1,
  config_script_id: null
})

const loading = ref(false)
const exploitList = ref<Array<{id: number, name: string}>>([])

// 正确解码包含UTF-8字符的base64字符串
const decodeBase64UTF8 = (base64: string): string => {
  try {
    // 使用decodeURIComponent和escape来处理UTF-8字符
    return decodeURIComponent(escape(atob(base64)))
  } catch (error) {
    console.error('Base64解码失败:', error)
    // 如果解码失败，尝试直接使用atob
    try {
      return atob(base64)
    } catch (e) {
      return base64 // 如果都失败了，返回原始字符串
    }
  }
}

// 获取exploit列表
const fetchExploitList = async () => {
  try {
    const params = new URLSearchParams()
    params.append('page_size', '1000') // 获取所有exploit
    
    const response = await fetch(`/webui/exploit_show?${params.toString()}`, {
      method: 'GET'
    })
    
    const result = await response.json()
    
    if (result.message === 'success' && result.result) {
      exploitList.value = result.result.map((item: any) => ({
        id: item.id,
        name: item.name
      }))
    } else {
      console.error('获取exploit列表失败:', result.error)
      exploitList.value = []
    }
  } catch (error) {
    console.error('获取exploit列表失败:', error)
    exploitList.value = []
  }
}

// 根据actionId获取数据
const fetchActionById = async (id: number | string) => {
  try {
    const response = await fetch(`/webui/action_get_by_id?id=${id.toString()}`, {
      method: 'GET'
    })
    
    const result = await response.json()
    
    if (result.message === 'success' && result.result) {
      return result.result
    } else {
      console.error('获取Action失败:', result.error)
      return null
    }
  } catch (error) {
    console.error('获取Action失败:', error)
    return null
  }
}

// 监听actionId变化，获取数据
watch(() => props.actionId, async (newId) => {
  if (newId && props.isEditing) {
    const action = await fetchActionById(newId)
    if (action) {
      updateFormFromAction(action)
    }
  }
}, { immediate: true })

// 监听action变化，更新表单
watch(() => props.action, (newAction) => {
  if (newAction) {
    updateFormFromAction(newAction)
  }
}, { immediate: true })

// 组件挂载时获取exploit列表
onMounted(() => {
  fetchExploitList()
})

// 从Action数据更新表单的通用函数
const updateFormFromAction = (action: Action) => {
  if (action) {
    // 解析config
    let configType = ''
    let configNum = 1
    let configScriptId = null
    if (action.config) {
      try {
        const config = JSON.parse(action.config)
        configType = config.type || ''
        configNum = config.num || 1
        configScriptId = config.script_id || null
      } catch (error) {
        console.error('解析config失败:', error)
      }
    }

    form.value = {
      id: action.id,
      name: action.name,
      code: action.code,
      output: action.output,
      error: action.error || '',
      interval: action.interval,
      timeout: action.timeout || 30,
      code_language: 'python3',
      next_run: action.next_run || '',
      config_type: configType,
      config_num: configNum,
      config_script_id: configScriptId
    }
    
    // 解析代码语言
    if (action.code && action.code.startsWith('data:code/')) {
      try {
        const parts = action.code.split(';base64,')
        if (parts.length === 2 && parts[0] && parts[1]) {
          const langPart = parts[0].split('/')
          if (langPart.length === 2 && langPart[1]) {
            // 先设置语言，再设置代码内容
            form.value.code_language = langPart[1]
          }
          
          // 解码base64
          const base64Code = parts[1]
          const decodedCode = decodeBase64UTF8(base64Code)
          form.value.code = decodedCode
        }
      } catch (error) {
        console.error('Base64解码失败:', error)
        form.value.code = action.code
      }
    } else if (action.code) {
      // 普通代码内容，尝试从代码内容推断语言
      form.value.code = action.code
      
      // 简单的语言推断逻辑
      if (action.code.includes('#!/usr/bin/env python2') || 
          action.code.includes('#!/usr/bin/python2')) {
        form.value.code_language = 'python2'
      } else if (action.code.includes('#!/usr/bin/env python3') || 
                 action.code.includes('#!/usr/bin/python3') ||
                 action.code.includes('print(')) {
        form.value.code_language = 'python3'
      } else if (action.code.includes('package main') || 
                 action.code.includes('func main()')) {
        form.value.code_language = 'golang'
      } else if (action.code.includes('#!/bin/bash') || 
                 action.code.includes('#!/usr/bin/env bash') ||
                 action.code.includes('echo ') ||
                 action.code.includes('$')) {
        form.value.code_language = 'bash'
      }
    }
  }
}

// 编码代码为base64格式
const encodeCodeToBase64 = (code: string, language: string): string => {
  if (!code.trim()) return ''
  try {
    // 使用encodeURIComponent和unescape来正确处理UTF-8字符
    const base64Code = btoa(unescape(encodeURIComponent(code)))
    return `data:code/${language};base64,${base64Code}`
  } catch (error) {
    console.error('Base64编码失败:', error)
    // 如果编码失败，尝试直接使用btoa
    try {
      const base64Code = btoa(code)
      return `data:code/${language};base64,${base64Code}`
    } catch (e) {
      return code // 如果都失败了，返回原始字符串
    }
  }
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

  // 如果是exec_script类型，验证必须选择脚本
  if (form.value.config_type === 'exec_script') {
    if (!form.value.config_script_id) {
      ElNotification({
        title: '验证失败',
        message: '请选择要执行的脚本',
        type: 'error',
        position: 'bottom-right'
      })
      return
    }
    if (!form.value.config_num || form.value.config_num <= 0) {
      ElNotification({
        title: '验证失败',
        message: '请输入有效的增加次数',
        type: 'error',
        position: 'bottom-right'
      })
      return
    }
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
    formData.append('timeout', form.value.timeout.toString())
    
    // 处理config
    let configStr = '{}'
    if (form.value.config_type) {
      const config: any = {
        type: form.value.config_type,
        num: form.value.config_num
      }
      // 如果是exec_script类型，添加script_id
      if (form.value.config_type === 'exec_script') {
        config.script_id = form.value.config_script_id
      }
      configStr = JSON.stringify(config)
    }
    formData.append('config', configStr)
    
    // 处理代码
    if (form.value.config_type === 'exec_script') {
      // exec_script类型不需要代码
      formData.append('code', '')
    } else if (form.value.code.trim()) {
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

// 关闭编辑
const closeEdit = () => {
  if (isStandalone.value) {
    // 如果是独立模式，关闭整个tab
    emit('close')
  } else {
    // 如果是对话框模式，关闭对话框
    dialogVisible.value = false
  }
}

// 重置表单
const resetForm = () => {
  form.value = {
    id: 0,
    name: '',
    code: '',
    output: '',
    error: '',
    interval: 0,
    timeout: 30,
    code_language: 'python3',
    next_run: '',
    config_type: '',
    config_num: 1,
    config_script_id: null
  }
}

// 格式化下次执行时间显示
const formatNextRunTime = (nextRun: string, interval: number) => {
  if (interval === 0) {
    return '不执行'
  } else if (interval === -1) {
    return '手动执行'
  } else if (nextRun) {
    try {
      const date = new Date(nextRun)
      return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })
    } catch (error) {
      return '时间格式错误'
    }
  } else {
    return '未设置'
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
                  -1: 手动执行, >=0: 间隔秒数（建议最低5秒）
                </div>
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
              <el-form-item label="超时时间">
                <el-input-number
                  v-model="form.timeout"
                  :min="0"
                  :max="60"
                  placeholder="超时时间（秒）"
                  style="width: 100%"
                />
                <div class="form-tip">
                  任务执行超时时间，0-60秒
                </div>
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12">
              <el-form-item label="配置类型">
                <el-select
                  v-model="form.config_type"
                  placeholder="选择配置类型"
                  style="width: 100%"
                  clearable
                >
                  <el-option label="无" value="" />
                  <el-option label="流量模版" value="template" />
                  <el-option label="Flag提交器" value="flag_submiter" />
                  <el-option label="执行脚本" value="exec_script" />
                </el-select>
                <div class="form-tip">
                  选择任务类型，用于特殊处理
                </div>
              </el-form-item>
            </el-col>
            
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12" v-if="form.config_type === 'flag_submiter'">
              <el-form-item label="Flag数量">
                <el-input-number
                  v-model="form.config_num"
                  :min="1"
                  :max="999"
                  placeholder="每次提交的flag数量"
                  style="width: 100%"
                />
                <div class="form-tip">
                  每次执行时提交的flag数量
                </div>
              </el-form-item>
            </el-col>
            
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12" v-if="form.config_type === 'exec_script'">
              <el-form-item label="执行脚本">
                <el-select
                  v-model="form.config_script_id"
                  placeholder="选择要执行的脚本"
                  style="width: 100%"
                  clearable
                >
                  <el-option
                    v-for="exploit in exploitList"
                    :key="exploit.id"
                    :label="exploit.name"
                    :value="exploit.id"
                  />
                </el-select>
                <div class="form-tip">
                  选择要增加运行次数的执行脚本
                </div>
              </el-form-item>
            </el-col>
            
            <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="12" v-if="form.config_type === 'exec_script'">
              <el-form-item label="增加次数">
                <el-input-number
                  v-model="form.config_num"
                  :min="1"
                  :max="1000"
                  placeholder="增加的运行次数"
                  style="width: 100%"
                />
                <div class="form-tip">
                  每次执行时增加的脚本运行次数
                </div>
              </el-form-item>
            </el-col>
          </el-row>
        </div>
        
        <div class="form-section" v-if="form.config_type !== 'exec_script'">
          <h3>代码内容</h3>
          <el-form-item label-width="0">
            <CodeEditor
              v-model="form.code"
              :language="form.code_language"
              @update:language="(lang: string) => form.code_language = lang"
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
        
        <div class="form-section" v-if="form.config_type === 'exec_script'">
          <h3>执行说明</h3>
          <el-form-item label-width="0">
            <el-alert
              title="执行脚本类型说明"
              type="info"
              :closable="false"
              show-icon
            >
              <template #default>
                <p>此类型的定时计划不需要编写代码，系统会自动增加指定执行脚本的运行次数。</p>
                <p>请在上方选择要操作的执行脚本和增加次数。</p>
              </template>
            </el-alert>
          </el-form-item>
        </div>
        
        <div class="form-section">
          <h3>错误信息</h3>
          <el-form-item label-width="0">
            <el-input
              v-model="form.error"
              type="textarea"
              :rows="3"
              placeholder="错误信息（只读）"
              maxlength="10000"
              show-word-limit
              readonly
              class="error-input"
            />
          </el-form-item>
          
          <h3>下次执行时间</h3>
          <el-form-item label-width="0">
            <el-input
              :value="formatNextRunTime(form.next_run, form.interval)"
              placeholder="下次执行时间（只读）"
              readonly
              class="next-run-input"
            />
          </el-form-item>
        </div>
      </el-form>
      
      <div class="edit-footer">
        <el-button @click="closeEdit">关闭</el-button>
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
      
      <h3>错误信息</h3>
      <el-form-item>
        <el-input
          v-model="form.error"
          type="textarea"
          :rows="3"
          placeholder="错误信息（只读）"
          maxlength="10000"
          show-word-limit
          readonly
          class="error-input"
        />
      </el-form-item>
      
      <h3>下次执行时间</h3>
      <el-form-item>
        <el-input
          :value="formatNextRunTime(form.next_run, form.interval)"
          placeholder="下次执行时间（只读）"
          readonly
          class="next-run-input"
        />
      </el-form-item>
    </el-form>
    
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="closeEdit">关闭</el-button>
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

.error-input :deep(.el-textarea__inner) {
  background-color: #fef0f0;
  border-color: #f56c6c;
  color: #f56c6c;
}

.next-run-input :deep(.el-input__inner) {
  background-color: #f0f9ff;
  border-color: #409eff;
  color: #409eff;
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
