<script setup lang="ts">
import { ElNotification, type ElForm, genFileId } from 'element-plus';
import { ref } from 'vue';
import type { UploadInstance, UploadProps, UploadRawFile } from 'element-plus'
import { useStore } from 'vuex'

const form = ref({
    exploit_uuid: '',
    environment: 'auto_pipreqs=True;',
    command: '',
    argv: '',
    platform: '',
    filter: '',
    times: '1'
})

const store = useStore()

const uploadRef = ref < UploadInstance > ()

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
    } else if (rawFile.size / 1024 / 1024 > 10) {
        ElNotification({
            title: 'Upload Failed',
            message: 'The file size cannot exceed 10MB!',
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

const fileChange = (file: UploadRawFile) => {
    let name = file.name
    if (name.endsWith('.zip') || name.endsWith('.tar')) {
        form.value.environment = 'auto_pipreqs=True;'
    }
}

const submit = () => {
    uploadRef.value?.submit()
}

const success_notice = (res:{
    message: string,
    exploit_uuid: string
}) => {
    ElNotification({
        title: 'Upload Success',
        message: res.message,
        type: 'success',
        position: 'bottom-right',
    })
    store.commit('push', res.exploit_uuid)
}

const error_notice = () => {
    ElNotification({
        title: 'Upload Failed',
        message: 'The server cannot respond to your request!',
        type: 'error',
        position: 'bottom-right',
    })
}

</script>

<template>
    <ElForm :inline="true" :label-width="120">
        <el-form-item label="Exploit UUID">
            <el-input name="exploit_uuid" v-model="form.exploit_uuid" />
        </el-form-item>
        <el-form-item label="Environment">
            <el-input name="environment" v-model="form.environment" />
        </el-form-item>
        <el-form-item label="Command">
            <el-input name="command" v-model="form.command" />
        </el-form-item>
        <el-form-item label="Argv">
            <el-input name="argv" v-model="form.argv" />
        </el-form-item>
        <el-form-item label="Platform">
            <el-input name="platform" v-model="form.platform" />
        </el-form-item>
        <el-form-item label="Filter">
            <el-input name="filter" v-model="form.filter" />
        </el-form-item>
        <el-form-item label="Times">
            <el-input-number :min="-2" :max="10" v-model="form.times" />
        </el-form-item>
    </ElForm>
    <el-upload class="upload-demo" drag action="/api/submit" :auto-upload="false" :data="form" ref="uploadRef"
        :on-success="success_notice" :before-upload="beforeUpload" :on-exceed="handleExceed" :on-change="fileChange"
        :on-error="error_notice">
        <el-icon class="el-icon--upload"><upload-filled /></el-icon>
        <div class="el-upload__text">
            拖动文件到此处,或者<em>点击此处上传</em>
        </div>
        <template #tip>
            <div class="el-upload__tip">
                只能上传一个Python文件或者压缩文件,且不超过10MB
            </div>
        </template>
    </el-upload>
    <el-button type="primary" class="ml-3" @click="submit">提交</el-button>
</template>

<style>
@media screen and (max-width: 430px) {
    .el-input {
        width: 56vw;
    }
}

@media screen and (min-width: 430px) {
    .el-input {
        width: 245px;
    }
}


.upload-demo {
    width: 100%;
    border-radius: 6px;
    cursor: pointer;
    box-sizing: border-box;
    padding: 20px 0;
    text-align: center;
}

.el-button {
    width: 100%;
}
</style>