<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { python } from '@codemirror/lang-python'
import { javascript } from '@codemirror/lang-javascript'
import { oneDark } from '@codemirror/theme-one-dark'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  },
  language: {
    type: String,
    default: 'python3'
  },
  placeholder: {
    type: String,
    default: '在此处编写代码...'
  }
})

const emit = defineEmits(['update:modelValue', 'update:language'])

const code = ref(props.modelValue)
const currentLanguage = ref(props.language)

const extensions: Record<string, any> = {
  'python2': python(),
  'python3': python(),
  'golang': javascript()
}

const languageOptions = [
  { label: 'Python 3', value: 'python3' },
  { label: 'Python 2', value: 'python2' },
  { label: 'Golang', value: 'golang' }
]

const handleChange = (value: string) => {
  code.value = value
  emit('update:modelValue', value)
}

const handleLanguageChange = (lang: string) => {
  currentLanguage.value = lang
  emit('update:language', lang)
}

watch(() => props.modelValue, (newValue) => {
  if (newValue !== code.value) {
    code.value = newValue
  }
})

watch(() => props.language, (newLang) => {
  if (newLang !== currentLanguage.value) {
    currentLanguage.value = newLang
  }
})

const getExtension = () => {
  return [extensions[currentLanguage.value] || python(), oneDark]
}
</script>

<template>
  <div class="code-editor-container">
    <div class="editor-header">
      <span class="editor-title">代码编辑器</span>
      <el-select 
        v-model="currentLanguage" 
        @change="handleLanguageChange"
        size="small"
        style="width: 100px;"
      >
        <el-option 
          v-for="option in languageOptions" 
          :key="option.value" 
          :label="option.label" 
          :value="option.value" 
        />
      </el-select>
    </div>
    
    <Codemirror
      v-model="code"
      :placeholder="placeholder"
      :style="{ height: '300px', width: '100%' }"
      :autofocus="true"
      :indent-with-tab="true"
      :tab-size="4"
      :extensions="getExtension()"
      @change="handleChange"
    />
  </div>
</template>

<style scoped>
.code-editor-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  margin-bottom: 20px;
  padding: 10px; /* 容器内边距 */
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 15px;
  background-color: #f5f7fa;
  border-bottom: 1px solid #dcdfe6;
  margin: -10px -10px 15px -10px; /* 调整头部边距以适应容器内边距 */
}

.editor-title {
  font-weight: bold;
  color: #606266;
}

:deep(.cm-editor) {
  height: 300px;
  font-size: 14px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

:deep(.cm-placeholder) {
  color: #909399;
  font-style: italic;
}
</style>