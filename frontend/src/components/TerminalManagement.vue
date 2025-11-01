<template>
  <div class="terminal-management">
    <div class="header">
      <h2>终端管理</h2>
      <div class="header-actions">
        <el-button type="primary" @click="refreshClients" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="success" @click="showTrafficCollectionDialog = true">
          <el-icon><Plus /></el-icon>
          下发流量采集
        </el-button>
      </div>
    </div>

    <!-- 客户端列表 -->
    <div class="clients-container">
      <el-card v-for="client in sortedClients" :key="client.id" class="client-card" shadow="hover">
        <template #header>
          <div class="client-header">
            <div class="client-info">
              <h3>{{ client.hostname || client.name }}</h3>
              <div class="client-meta">
                <el-tag size="small" :type="getPlatformType(client.platform)">
                  {{ client.platform }}/{{ client.arch }}
                </el-tag>
                <el-tag size="small" type="info">ID: {{ client.id }}</el-tag>
                <el-tag size="small" type="success">在线</el-tag>
              </div>
            </div>
            <div class="client-stats">
              <div class="stat-item">
                <span class="stat-label">CPU:</span>
                <span class="stat-value">{{ client.cpu_use }}%</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">内存:</span>
                <span class="stat-value">{{ client.memory_use }}MB/{{ client.memory_max }}MB</span>
              </div>
            </div>
          </div>
        </template>

        <div class="client-content">
          <!-- 网卡信息 -->
          <div class="interfaces-section">
            <h4>网络接口</h4>
            <div v-if="client.interfaces && client.interfaces.length > 0" class="interfaces-grid">
              <div 
                v-for="(iface, index) in client.interfaces" 
                :key="index"
                class="interface-item"
                @click="selectInterface(client, iface)"
                @dblclick="openTrafficDialogForInterface(client, iface)"
                :class="{ selected: selectedInterface && selectedInterface.clientId === client.id && selectedInterface.interfaceName === iface.name }"
              >
                <div class="interface-main">
                  <span class="interface-name">{{ iface.name }}</span>
                  <span class="interface-ip" v-if="iface.ip">{{ iface.ip }}</span>
                </div>
                <div class="interface-desc" v-if="iface.description">{{ iface.description }}</div>
              </div>
            </div>
            <div v-else class="no-interfaces">
              <el-empty description="暂无网卡信息" :image-size="60" />
            </div>
          </div>

          <!-- 监控任务 -->
          <div class="monitors-section">
            <h4>监控任务</h4>
            <div v-if="client.monitors && client.monitors.length > 0" class="monitors-list">
              <div 
                v-for="monitor in client.monitors" 
                :key="monitor.id"
                class="monitor-item"
              >
                <div class="monitor-info">
                  <span class="monitor-name">{{ monitor.name || '全部网卡' }}</span>
                  <el-tag size="small" type="primary">{{ monitor.types }}</el-tag>
                  <el-tag size="small" type="info">{{ monitor.interval }}s</el-tag>
                </div>
                <div class="monitor-actions">
                  <el-button size="small" type="danger" @click="deleteMonitor(monitor.id)">
                    删除
                  </el-button>
                </div>
              </div>
            </div>
            <div v-else class="no-monitors">
              <el-empty description="暂无监控任务" :image-size="50" />
            </div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 流量采集对话框 -->
    <el-dialog
      v-model="showTrafficCollectionDialog"
      title="下发流量采集任务"
      width="600px"
      :before-close="handleDialogClose"
    >
      <el-form :model="trafficForm" :rules="trafficRules" ref="trafficFormRef" label-width="120px">
        <el-form-item label="选择客户端" prop="clientId">
          <el-select v-model="trafficForm.clientId" placeholder="请选择客户端" style="width: 100%">
            <el-option
              v-for="client in sortedClients"
              :key="client.id"
              :label="`${client.hostname || client.name} (ID: ${client.id})`"
              :value="client.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="选择网卡" prop="interfaceName">
          <el-select v-model="trafficForm.interfaceName" placeholder="请选择网卡" style="width: 100%">
            <el-option label="全部网卡" value="" />
            <template v-for="client in sortedClients" :key="client.id">
              <el-option
                v-if="client.id === trafficForm.clientId"
                v-for="iface in client.interfaces"
                :key="`${client.id}-${iface.name}`"
                :label="iface.description ? `${iface.name} - ${iface.description}` : iface.name"
                :value="iface.name"
              />
            </template>
          </el-select>
        </el-form-item>

        <el-form-item label="BPF过滤器" prop="bpf">
          <el-input
            v-model="trafficForm.bpf"
            type="textarea"
            :rows="3"
            placeholder="留空表示采集所有流量，例如: tcp port 80"
          />
        </el-form-item>

        <el-form-item label="采集间隔" prop="interval">
          <el-input-number
            v-model="trafficForm.interval"
            :min="1"
            :max="3600"
            :step="1"
            style="width: 200px"
          />
          <span style="margin-left: 10px">秒</span>
        </el-form-item>

        <el-form-item label="任务描述" prop="description">
          <el-input
            v-model="trafficForm.description"
            type="textarea"
            :rows="3"
            placeholder="可选，用于描述此采集任务"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showTrafficCollectionDialog = false">取消</el-button>
          <el-button type="primary" @click="submitTrafficCollection" :loading="submitting">
            下发任务
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus } from '@element-plus/icons-vue'

interface NetworkInterface {
  name: string
  description?: string
  ip?: string
}

interface Monitor {
  id: number
  client_id: number
  name: string
  types: string
  data: string
  interval: number
  created_at: string
  updated_at: string
}

interface Client {
  id: number
  name: string
  hostname: string
  platform: string
  arch: string
  cpu: string
  cpu_use: string
  memory_use: string
  memory_max: string
  updated_at: string
  interfaces: NetworkInterface[]
  monitors?: Monitor[]
}

interface SelectedInterface {
  clientId: number
  interfaceName: string
}

const clients = ref<Client[]>([])
const loading = ref(false)
const showTrafficCollectionDialog = ref(false)
const submitting = ref(false)
const selectedInterface = ref<SelectedInterface | null>(null)

const trafficForm = reactive({
  clientId: null as number | null,
  interfaceName: '',
  bpf: '',
  interval: 60,
  description: ''
})

const trafficRules = {
  clientId: [
    { required: true, message: '请选择客户端', trigger: 'change' }
  ],
  interval: [
    { required: true, message: '请设置采集间隔', trigger: 'blur' }
  ]
}

const trafficFormRef = ref()

// 按ID排序的客户端列表
const sortedClients = computed(() => {
  return [...clients.value].sort((a, b) => a.id - b.id)
})

// 获取平台类型标签
const getPlatformType = (platform: string) => {
  switch (platform.toLowerCase()) {
    case 'windows':
      return 'danger'
    case 'linux':
      return 'success'
    case 'darwin':
      return 'warning'
    default:
      return 'info'
  }
}

// 刷新客户端列表
const refreshClients = async () => {
  loading.value = true
  try {
    const response = await fetch('/webui/clients', {
      method: 'GET'
    })
    const result = await response.json()
    
    if (result.message === 'success') {
      clients.value = result.result
      
      // 为每个客户端加载监控任务
      for (const client of clients.value) {
        await loadClientMonitors(client.id)
      }
    } else {
      ElMessage.error('获取客户端列表失败: ' + result.error)
    }
  } catch (error) {
    ElMessage.error('获取客户端列表失败: ' + error)
  } finally {
    loading.value = false
  }
}

// 加载客户端的监控任务
const loadClientMonitors = async (clientId: number) => {
  try {
    const formData = new FormData()
    formData.append('client_id', clientId.toString())
    
    const response = await fetch('/webui/client_monitors', {
      method: 'POST',
      body: formData
    })
    const result = await response.json()
    
    if (result.message === 'success') {
      const client = clients.value.find(c => c.id === clientId)
      if (client) {
        client.monitors = result.result
      }
    }
  } catch (error) {
    console.error('加载监控任务失败:', error)
  }
}

// 选择网卡
const selectInterface = (client: Client, iface: NetworkInterface) => {
  selectedInterface.value = {
    clientId: client.id,
    interfaceName: iface.name
  }
}

// 为指定网卡打开流量采集对话框
const openTrafficDialogForInterface = (client: Client, iface: NetworkInterface) => {
  // 预填充表单
  trafficForm.clientId = client.id
  trafficForm.interfaceName = iface.name
  // 将网卡描述预填充到任务描述中
  if (iface.description) {
    trafficForm.description = `监控网卡: ${iface.name} (${iface.description})`
  } else {
    trafficForm.description = `监控网卡: ${iface.name}`
  }
  // 重置其他字段为默认值（保持BPF和间隔为默认）
  trafficForm.bpf = ''
  trafficForm.interval = 60
  
  // 打开对话框
  showTrafficCollectionDialog.value = true
  
  // 等待对话框打开后，验证表单（确保选择的下拉框正确显示）
  setTimeout(() => {
    trafficFormRef.value?.clearValidate()
  }, 100)
}

// 删除监控任务
const deleteMonitor = async (monitorId: number) => {
  try {
    await ElMessageBox.confirm('确定要删除此监控任务吗？', '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    const formData = new FormData()
    formData.append('monitor_id', monitorId.toString())

    const response = await fetch('/webui/delete_monitor', {
      method: 'POST',
      body: formData
    })

    const result = await response.json()
    
    if (result.message === 'success') {
      ElMessage.success('监控任务删除成功')
      // 刷新客户端列表
      await refreshClients()
    } else {
      ElMessage.error('删除监控任务失败: ' + result.error)
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除监控任务失败: ' + error)
    }
  }
}

// 提交流量采集任务
const submitTrafficCollection = async () => {
  if (!trafficFormRef.value) return
  
  try {
    await trafficFormRef.value.validate()
    
    submitting.value = true
    
    const formData = new FormData()
    formData.append('client_id', trafficForm.clientId!.toString())
    formData.append('interface_name', trafficForm.interfaceName)
    formData.append('bpf', trafficForm.bpf)
    formData.append('interval', trafficForm.interval.toString())
    formData.append('description', trafficForm.description)

    const response = await fetch('/webui/traffic_collection', {
      method: 'POST',
      body: formData
    })

    const result = await response.json()
    
    if (result.message === 'success') {
      ElMessage.success('流量采集任务下发成功')
      showTrafficCollectionDialog.value = false
      resetTrafficForm()
      // 刷新客户端列表
      await refreshClients()
    } else {
      ElMessage.error('下发流量采集任务失败: ' + result.error)
    }
  } catch (error) {
    ElMessage.error('下发流量采集任务失败: ' + error)
  } finally {
    submitting.value = false
  }
}

// 重置表单
const resetTrafficForm = () => {
  trafficForm.clientId = null
  trafficForm.interfaceName = ''
  trafficForm.bpf = ''
  trafficForm.interval = 60
  trafficForm.description = ''
  trafficFormRef.value?.resetFields()
}

// 关闭对话框
const handleDialogClose = () => {
  resetTrafficForm()
  showTrafficCollectionDialog.value = false
}

// 组件挂载时加载数据
onMounted(() => {
  refreshClients()
})
</script>

<style scoped>
.terminal-management {
  padding: 16px;
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.header h2 {
  margin: 0;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.clients-container {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 16px;
  flex: 1;
  align-content: start;
  overflow-y: auto;
}

.client-card {
  margin-bottom: 0;
  display: flex;
  flex-direction: column;
  height: fit-content;
}

.client-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.client-info h3 {
  margin: 0 0 8px 0;
  color: #303133;
  font-size: 16px;
}

.client-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.client-stats {
  display: flex;
  flex-direction: column;
  gap: 4px;
  text-align: right;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.stat-label {
  color: #909399;
}

.stat-value {
  color: #303133;
  font-weight: 500;
}

.client-content {
  margin-top: 12px;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.interfaces-section,
.monitors-section {
  margin-bottom: 12px;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.interfaces-section:last-child,
.monitors-section:last-child {
  margin-bottom: 0;
}

.interfaces-section h4,
.monitors-section h4 {
  margin: 0 0 8px 0;
  color: #606266;
  font-size: 13px;
  font-weight: 600;
}

.interfaces-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px;
  flex: 1;
}

.interface-item {
  padding: 8px 10px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  background: #fafafa;
}

.interface-item:hover {
  border-color: #409eff;
  background: #f0f9ff;
}

.interface-item.selected {
  border-color: #409eff;
  background: #e6f7ff;
}

.interface-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2px;
}

.interface-name {
  font-weight: 600;
  color: #303133;
  font-size: 13px;
}

.interface-desc {
  font-size: 11px;
  color: #909399;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.interface-ip {
  font-size: 11px;
  color: #67c23a;
  font-family: monospace;
  background: #f0f9ff;
  padding: 1px 4px;
  border-radius: 2px;
}

.no-interfaces,
.no-monitors {
  text-align: center;
  padding: 20px;
  color: #909399;
}

.monitors-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.monitor-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 10px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  background: #fafafa;
}

.monitor-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
}

.monitor-name {
  font-weight: 500;
  color: #303133;
  font-size: 13px;
  margin-right: 6px;
}

.monitor-actions {
  display: flex;
  gap: 6px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* 响应式设计 */
@media (max-width: 1400px) {
  .clients-container {
    grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
  }
}

@media (max-width: 1024px) {
  .clients-container {
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  }
  
  .interfaces-grid {
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  }
}

@media (max-width: 768px) {
  .terminal-management {
    padding: 12px;
  }
  
  .clients-container {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  
  .client-header {
    flex-direction: column;
    gap: 12px;
  }
  
  .client-stats {
    text-align: left;
  }
  
  .interfaces-grid {
    grid-template-columns: 1fr;
  }
}
</style>
