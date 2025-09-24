<script setup lang="ts">
import { ElNotification, type ElForm, genFileId } from 'element-plus';
import { ref, computed, watch, onMounted } from 'vue';
import type { UploadInstance, UploadProps, UploadRawFile } from 'element-plus'
import { useStore } from 'vuex'
import CodeEditor from './CodeEditor.vue'

const form = ref({
    exploit_uuid: '',
    environment: '',
    command: '',
    argv: '',
    platform: '',
    arch: '',
    filter: '',
    times: 0,
    flag: '',
    code: '',
    code_language: 'python3',
    usePipreqs: true,
    filename: '' // 添加文件名字段
})

const hasFile = ref(false)
const activeTab = ref('code') // 'code' or 'file' - 代码编辑在前

const store = useStore()

const uploadRef = ref<UploadInstance>()

// 计算最终的environment值
const computedEnvironment = computed(() => {
    let env = form.value.environment
    if (form.value.usePipreqs) {
        if (env && !env.endsWith(';')) {
            env += ';'
        }
        env += 'auto_pipreqs=True;'
    }
    return env
})

// 计算文件上传的数据
const uploadData = computed(() => {
    return {
        ...form.value,
        environment: computedEnvironment.value
    }
})

const beforeUpload: UploadProps['beforeUpload'] = (rawFile) => {
    let name = rawFile.name
    if (name.endsWith('.py') && name.endsWith('.zip')
        && name.endsWith('.tar')) {
        ElNotification({
            title: 'Upload Failed',
            message: 'You can only upload one Python file or compressed file!',
            type: 'error',
            position: 'bottom-right',
        })
        return false
    } else if (rawFile.size / 1024 / 1024 > 1024) {
        ElNotification({
            title: 'Upload Failed',
            message: 'The file size cannot exceed 1024MB!',
            type: 'error',
            position: 'bottom-right',
        })
        return false
    }
    return true
}

const handleExceed: UploadProps['onExceed'] = (files) => {
    uploadRef.value!.clearFiles()
    const file = files[0] as UploadRawFile
    file.uid = genFileId()
    uploadRef.value!.handleStart(file)
}

const fileChange:UploadProps['onChange'] = (file, files) => {
    hasFile.value = files.length > 0
    let name = file.name
    if (name.endsWith('.zip') || name.endsWith('.tar')) {
        // 压缩文件时默认启用pipreqs
        form.value.usePipreqs = true
    }
}

// 修改文件上传的data绑定，使用计算后的environment
const updateUploadData = () => {
    if (uploadRef.value) {
        const uploadElement = uploadRef.value
        if ('setData' in uploadElement) {
          (uploadElement as any).setData({
            ...form.value,
            environment: computedEnvironment.value
          })
        }
    }
}

const fileRemove: UploadProps['onRemove'] = (file, files) => {
    hasFile.value = files.length > 0
}

const switchTab = (tab: string) => {
    activeTab.value = tab
    if (tab === 'file') {
        form.value.code = ''
    } else {
        hasFile.value = false
        uploadRef.value?.clearFiles()
    }
}

// 监听相关字段变化，更新上传数据
watch(() => [form.value.usePipreqs, form.value.environment], () => {
    updateUploadData()
}, { immediate: true })

const encodeCodeToBase64 = (code: string, language: string): string => {
    const base64Code = btoa(unescape(encodeURIComponent(code)))
    return `data:code/${language};base64,${base64Code}`
}

const hasContent = computed(() => {
    return hasFile.value || form.value.command !== '' || form.value.code !== ''
})

const submit = () => {
    if (!hasContent.value) {
        ElNotification({
            title: '错误',
            message: '请先上传文件、输入命令或编写代码！',
            type: 'error',
            position: 'bottom-right',
        })
        return
    }

    // 准备提交数据
    const submitData = {
        ...form.value,
        environment: computedEnvironment.value,
        platform: form.value.platform,
        arch: form.value.arch
    }

    if (hasFile.value) {
        // 文件上传模式
        uploadRef.value?.submit()
    } else if (form.value.code) {
        // 代码模式
        const formData = new FormData();
        for (const [key, value] of Object.entries(submitData)) {
            if (key === 'code') {
                // 将代码编码为base64格式
                const encodedCode = encodeCodeToBase64(String(value), form.value.code_language)
                formData.append('code', encodedCode)
            } else {
                formData.append(key, String(value));
            }
        }
        
        fetch('/webui/exploit', {
            method: 'POST',
            body: formData
        }).then(res => res.json()).then(res => {
            success_notice(res)
            refreshResults() // 提交成功后刷新结果
        }).catch(_err => {
            error_notice()
        })
    } else if (form.value.command !== '') {
        // 命令模式
        const formData = new FormData();
        for (const [key, value] of Object.entries(submitData)) {
            formData.append(key, String(value));
        }
        fetch('/webui/exploit', {
            method: 'POST',
            body: formData
        }).then(res => res.json()).then(res => {
            success_notice(res)
            refreshResults() // 提交成功后刷新结果
        }).catch(_err => {
            error_notice()
        })
    }
}

// 刷新结果数据
const refreshResults = () => {
    // 触发Vuex action来刷新数据
    store.dispatch('fetchResults', { page: 1, pageSize: 20 }).catch(error => {
        console.error('刷新结果失败:', error)
    })
}

const success_notice = (res: {
    message: string,
    exploit_uuid: string
}) => {
    ElNotification({
        title: '上传成功',
        message: res.message,
        type: 'success',
        position: 'bottom-right',
    })
    
    // 自动将返回的UUID填充到输入框中
    if (res.exploit_uuid) {
        form.value.exploit_uuid = res.exploit_uuid
        // 更新URL
        const url = new URL(window.location.href)
        url.searchParams.set('uuid', res.exploit_uuid)
        window.history.pushState({ path: url.href }, '', url.href)
    }
    
    // 文件上传成功后重置文件状态
    if (hasFile.value) {
        hasFile.value = false
        uploadRef.value?.clearFiles()
    }
    
    store.commit('push', res.exploit_uuid)
    refreshResults() // 刷新结果
}

const error_notice = () => {
    ElNotification({
        title: '上传失败',
        message: '服务器无法响应您的请求！',
        type: 'error',
        position: 'bottom-right',
    })
}

onMounted(() => {
    // 从URL中获取UUID
    const urlParams = new URLSearchParams(window.location.search)
    const uuid = urlParams.get('uuid')
    if (uuid) {
        form.value.exploit_uuid = uuid
        refreshResults()
        
        // 调用 exploit_get_by_uuid 获取任务详细信息
        fetch('/webui/exploit_get_by_uuid', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: `uuid=${uuid}`
        })
        .then(res => res.json())
        .then(res => {
            if (res.message === 'success' && res.result) {
                const task = res.result
                
                // 处理环境变量，移除 auto_pipreqs=True;
                if (task.environment) {
                    form.value.environment = task.environment
                        .replace(/auto_pipreqs=True;/g, '')
                        .replace(/;{2,}/g, ';') // 移除连续的分号
                        .replace(/^;|;$/g, '') // 移除开头和结尾的分号
                        .trim()
                } else {
                    form.value.environment = ''
                }
                
                form.value.command = task.command || ''
                form.value.argv = task.argv || ''
                form.value.platform = task.platform || ''
                form.value.arch = task.arch || ''
                form.value.filter = task.filter || ''
                form.value.times = task.times || 0 // 默认值为0
                form.value.flag = task.flag || ''
                
                // 设置文件名（如果存在）
                if (task.filename && !task.filename.startsWith('data:code/')) {
                    form.value.filename = task.filename
                } else {
                    form.value.filename = ''
                }
                
                // 处理 base64 编码的代码
                if (task.code && task.code.startsWith('data:code/')) {
                    try {
                        // 解析 data:code/python3;base64,ZXJhcmU= 格式
                        const parts = task.code.split(';base64,')
                        if (parts.length === 2) {
                            // 获取语言类型（由前端解析，不需要后端返回）
                            const langPart = parts[0].split('/')
                            if (langPart.length === 2) {
                                form.value.code_language = langPart[1]
                            }
                            
                            // 解码 base64
                            const base64Code = parts[1]
                            const decodedCode = atob(base64Code)
                            form.value.code = decodedCode
                            activeTab.value = 'code'
                        }
                    } catch (error) {
                        console.error('Base64 解码失败:', error)
                        form.value.code = task.code // 如果解码失败，保持原始内容
                    }
                } else if (task.code) {
                    // 普通代码内容
                    form.value.code = task.code
                    activeTab.value = 'code'
                    // 对于普通代码，使用默认语言或保持当前设置
                } else {
                    form.value.code = ''
                }
            }
        })
        .catch(error => {
            console.error('获取任务详情失败:', error)
        })
    }
})
</script>

<template>
    <ElForm :inline="true" :label-width="80">
        <el-form-item label="UUID">
            <el-input name="exploit_uuid" v-model="form.exploit_uuid" placeholder="请输入UUID" />
        </el-form-item>
        <el-form-item label="环境">
            <el-input name="environment" v-model="form.environment" placeholder="环境变量设置" />
        </el-form-item>
        <el-form-item label="运行命令">
            <el-input name="command" v-model="form.command" placeholder="要执行的命令" />
        </el-form-item>
        <el-form-item label="参数">
            <el-input name="argv" v-model="form.argv" placeholder="命令行参数" />
        </el-form-item>
        
        <!-- OS选择器 - 一个框内左右分开，保持原有宽度 -->
        <el-form-item label="系统">
            <div class="os-selectors">
                <el-select v-model="form.platform" placeholder="平台" class="platform-selector">
                    <el-option label="全部" value="" />
                    <el-option label="Windows" value="windows" />
                    <el-option label="FreeBSD" value="freebsd" />
                    <el-option label="Linux" value="linux" />
                    <el-option label="macOS" value="darwin" />
                </el-select>
                <el-select v-model="form.arch" placeholder="架构" class="arch-selector">
                    <el-option label="全部" value="" />
                    <el-option label="x64" value="amd64" />
                    <el-option label="x86" value="386" />
                    <el-option label="ARM64" value="arm64" />
                </el-select>
            </div>
        </el-form-item>
        
        <el-form-item label="筛选">
            <el-input name="filter" v-model="form.filter" placeholder="筛选条件" />
        </el-form-item>
        <el-form-item label="Flag正则">
            <el-input name="flag" v-model="form.flag" placeholder="Flag匹配" />
        </el-form-item>
        <el-form-item label="执行次数">
            <el-input-number :min="-2" :max="999" v-model="form.times" class="el-input-number" />
        </el-form-item>
        <el-form-item label="依赖">
            <el-checkbox v-model="form.usePipreqs">自动安装</el-checkbox>
        </el-form-item>
    </ElForm>
    
    <el-tabs v-model="activeTab" type="card" @tab-change="switchTab" class="mode-tabs">
        <el-tab-pane name="code" label="代码编辑">
            <CodeEditor 
                v-model="form.code" 
                :language="form.code_language"
                @update:language="(lang) => form.code_language = lang"
                placeholder="print('Hello world')"
            />
        </el-tab-pane>
        
        <el-tab-pane name="file" label="文件上传">
            <el-upload class="upload-demo" drag action="/webui/exploit" :auto-upload="false" :data="uploadData" ref="uploadRef"
                :on-success="success_notice" :before-upload="beforeUpload" :on-exceed="handleExceed" :on-change="fileChange"
                :on-error="error_notice" :on-remove="fileRemove" :multiple="false">
                <el-icon class="el-icon--upload"><upload-filled /></el-icon>
                <div class="el-upload__text">
                    拖动文件到此处,或者<em>点击此处上传</em>
                </div>
            </el-upload>
        </el-tab-pane>
    </el-tabs>
    
    <el-button type="primary" class="ml-3 submit-btn" @click="submit">提交</el-button>
</template>

<style>
@media screen and (max-width: 430px) {

    .el-input,
    .el-input-number {
        width: 56vw !important;
    }
}

@media screen and (min-width: 430px) {

    .el-input,
    .el-input-number {
        width: 200px !important;
    }
}

.mode-tabs {
    margin-bottom: 20px;
    width: 100%;
}

.mode-tabs .el-tabs__content {
    padding: 0;
    width: 100%;
    min-height: 400px;
}

.mode-tabs .el-tab-pane {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
}

/* 确保 tabs 容器和内容区域充分利用空间 */
.mode-tabs .el-tabs__header {
    width: 100%;
}

.mode-tabs .el-tabs__nav-wrap {
    width: 100%;
}

.mode-tabs .el-tabs__nav-scroll {
    width: 100%;
}

/* 代码编辑器响应式样式 */
.mode-tabs .el-tab-pane .code-editor-container {
    width: 100%;
    flex: 1;
    min-height: 400px;
    display: flex;
    flex-direction: column;
}

.mode-tabs .el-tab-pane .code-editor-container .cm-editor {
    flex: 1;
    min-height: 400px;
    width: 100%;
}

/* 文件上传区域响应式样式 */
.mode-tabs .el-tab-pane .upload-demo {
    width: 100%;
    min-height: 400px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
}

/* 响应式媒体查询 */
@media (max-width: 768px) {
    .mode-tabs .el-tabs__content {
        min-height: 300px;
    }
    
    .mode-tabs .el-tab-pane .code-editor-container {
        min-height: 300px;
    }
    
    .mode-tabs .el-tab-pane .code-editor-container .cm-editor {
        min-height: 300px;
    }
    
    .mode-tabs .el-tab-pane .upload-demo {
        min-height: 300px;
    }
}

@media (max-width: 480px) {
    .mode-tabs .el-tabs__content {
        min-height: 250px;
    }
    
    .mode-tabs .el-tab-pane .code-editor-container {
        min-height: 250px;
    }
    
    .mode-tabs .el-tab-pane .code-editor-container .cm-editor {
        min-height: 250px;
    }
    
    .mode-tabs .el-tab-pane .upload-demo {
        min-height: 250px;
    }
}

.upload-demo {
    width: 100%;
    border-radius: 6px;
    cursor: pointer;
    box-sizing: border-box;
    padding: 20px 0;
    text-align: center;
    border: 2px dashed #dcdfe6;
    background: #fafafa;
}

.upload-demo:hover {
    border-color: #409eff;
    background: #f0f7ff;
}

.submit-btn {
    width: 100%;
    margin-top: 20px;
    height: 40px;
    font-size: 16px;
}

.code-editor-container {
    border: 1px solid #e6e8eb;
    border-radius: 6px;
    margin-bottom: 20px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* 表单样式优化 */
.el-form {
    background: #fff;
    padding: 20px;
    border-radius: 6px;
    border: 1px solid #e6e8eb;
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
    margin-bottom: 20px;
}

.el-form-item {
    margin-bottom: 18px;
}

.el-form-item__label {
    font-weight: 600;
    color: #303133;
    font-size: 14px;
}

.el-input .el-input__wrapper,
.el-select .el-input__wrapper {
    border-radius: 4px;
    box-shadow: 0 0 0 1px #dcdfe6 inset;
}

.el-input .el-input__wrapper:hover,
.el-select .el-input__wrapper:hover {
    box-shadow: 0 0 0 1px #409eff inset;
}

.el-input .el-input__wrapper.is-focus,
.el-select .el-input__wrapper.is-focus {
    box-shadow: 0 0 0 1px #409eff inset;
}

.el-checkbox {
    margin-right: 0;
}

.el-checkbox__label {
    color: #606266;
    font-size: 14px;
}

/* OS选择器样式 - 一个框内左右分开，保持原有宽度 */
.os-selectors {
    display: flex;
    gap: 0;
    align-items: center;
    border: 1px solid #dcdfe6;
    border-radius: 4px;
    overflow: hidden;
    background: #fff;
    width: 200px; /* 保持和原来单个选择器相同的宽度 */
}

.platform-selector,
.arch-selector {
    flex: 1;
    min-width: 0; /* 允许缩小 */
    border: none;
    border-radius: 0;
}

.platform-selector {
    border-right: 1px solid #dcdfe6;
}

.platform-selector .el-input__wrapper,
.arch-selector .el-input__wrapper {
    box-shadow: none !important;
    border: none !important;
}

.platform-selector .el-input__wrapper:hover,
.arch-selector .el-input__wrapper:hover {
    background-color: #f5f7fa;
}

/* 移除选择器的边框和阴影 */
.platform-selector .el-select,
.arch-selector .el-select {
    width: 100%;
}

.platform-selector .el-select .el-input,
.arch-selector .el-select .el-input {
    border: none;
}

/* 响应式调整 */
@media screen and (max-width: 768px) {
    .os-selectors {
        flex-direction: column;
        gap: 0;
        width: 100%; /* 小屏幕时恢复100%宽度 */
    }
    
    .platform-selector {
        border-right: none;
        border-bottom: 1px solid #dcdfe6;
    }
    
    .platform-selector,
    .arch-selector {
        width: 100%;
    }
}

/* 移动端适配 */
@media screen and (max-width: 430px) {
    .os-selectors {
        width: 56vw !important;
    }
    
    .el-form {
        padding: 15px;
    }
    
    .el-form-item {
        margin-bottom: 15px;
    }
}
</style>