<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import ActionList from './ActionList.vue'
import ExploitList from './ExploitList.vue'
import ActionEdit from './ActionEdit.vue'
import ExploitEdit from './ExploitEdit.vue'

interface Tab {
  id: string
  title: string
  type: 'action-list' | 'exploit-list' | 'action-edit' | 'exploit-edit'
  data?: any
  closable: boolean
  // 分页状态
  pagination?: {
    currentPage: number
    pageSize: number
    totalItems: number
  }
  // 搜索状态
  search?: {
    name?: string
    platform?: string
    arch?: string
  }
}

const tabs = ref<Tab[]>([
  {
    id: 'action-list',
    title: '定时计划',
    type: 'action-list',
    closable: false
  },
  {
    id: 'exploit-list',
    title: '执行脚本',
    type: 'exploit-list',
    closable: false
  }
])

const activeTabId = ref('action-list')
const isCollapsed = ref(false)
const isMobile = ref(false)

// 标签页持久化相关
const STORAGE_KEY = 'tab-manager-state'

// 保存状态到 localStorage
const saveState = () => {
  const state = {
    tabs: tabs.value,
    activeTabId: activeTabId.value,
    isCollapsed: isCollapsed.value
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

// 从 localStorage 恢复状态
const loadState = () => {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      const state = JSON.parse(saved)
      
      // 恢复标签页
      if (state.tabs && Array.isArray(state.tabs)) {
        tabs.value = state.tabs
      }
      
      // 恢复活动标签页
      if (state.activeTabId) {
        activeTabId.value = state.activeTabId
      }
      
      // 恢复折叠状态
      if (typeof state.isCollapsed === 'boolean') {
        isCollapsed.value = state.isCollapsed
      }
    }
  } catch (error) {
    console.warn('Failed to load tab state:', error)
  }
}

// 添加新选项卡
const addTab = (tab: Omit<Tab, 'id'>) => {
  const id = `${tab.type}-${Date.now()}`
  const newTab: Tab = {
    id,
    ...tab
  }
  tabs.value.push(newTab)
  activeTabId.value = id
}

// 关闭选项卡
const closeTab = (tabId: string) => {
  const index = tabs.value.findIndex(tab => tab.id === tabId)
  if (index > -1 && tabs.value[index].closable) {
    tabs.value.splice(index, 1)
    
    // 如果关闭的是当前活动选项卡，切换到其他选项卡
    if (activeTabId.value === tabId) {
      if (tabs.value.length > 0) {
        activeTabId.value = tabs.value[Math.max(0, index - 1)].id
      }
    }
  }
}

// 关闭所有标签页
const closeAllTabs = () => {
  // 保存当前标签页状态
  saveCurrentTabState()
  
  // 只保留不可关闭的标签页（通常是列表页面）
  tabs.value = tabs.value.filter(tab => !tab.closable)
  
  // 切换到第一个标签页
  if (tabs.value.length > 0) {
    activeTabId.value = tabs.value[0].id
    restoreTabState(activeTabId.value)
  }
}

// 暴露方法给父组件
defineExpose({
  closeAllTabs
})

// 切换到指定选项卡
const switchTab = (tabId: string) => {
  // 保存当前标签页的状态
  saveCurrentTabState()
  
  // 切换到新标签页
  activeTabId.value = tabId
  
  // 恢复新标签页的状态
  restoreTabState(tabId)
}

// 保存当前标签页状态
const saveCurrentTabState = () => {
  const currentTab = tabs.value.find(tab => tab.id === activeTabId.value)
  if (currentTab && (currentTab.type === 'action-list' || currentTab.type === 'exploit-list')) {
    // 保存分页状态到 localStorage
    const stateKey = `tab-state-${currentTab.id}`
    const state = {
      pagination: currentTab.pagination || { currentPage: 1, pageSize: 20, totalItems: 0 },
      search: currentTab.search || {},
      timestamp: Date.now()
    }
    localStorage.setItem(stateKey, JSON.stringify(state))
  }
}

// 恢复标签页状态
const restoreTabState = (tabId: string) => {
  const tab = tabs.value.find(t => t.id === tabId)
  if (tab && (tab.type === 'action-list' || tab.type === 'exploit-list')) {
    // 从 localStorage 恢复状态
    const stateKey = `tab-state-${tabId}`
    const savedState = localStorage.getItem(stateKey)
    if (savedState) {
      try {
        const state = JSON.parse(savedState)
        // 检查状态是否过期（24小时）
        if (Date.now() - state.timestamp < 24 * 60 * 60 * 1000) {
          tab.pagination = state.pagination
          tab.search = state.search
        }
      } catch (error) {
        console.error('恢复标签页状态失败:', error)
      }
    }
  }
}

// 获取当前活动选项卡
const activeTab = computed(() => {
  return tabs.value.find(tab => tab.id === activeTabId.value)
})

// 处理ActionList的事件
const handleActionEdit = (action: any) => {
  addTab({
    title: `${action.name} - 编辑定时计划`,
    type: 'action-edit',
    data: action,
    closable: true
  })
}

const handleActionAdd = () => {
  addTab({
    title: '新增定时计划',
    type: 'action-edit',
    data: null,
    closable: true
  })
}

// 处理ExploitList的事件
const handleExploitEdit = (exploit: any) => {
  addTab({
    title: `${exploit.exploit_uuid} - 编辑执行脚本`,
    type: 'exploit-edit',
    data: exploit,
    closable: true
  })
}

const handleExploitAdd = () => {
  addTab({
    title: '新增执行脚本',
    type: 'exploit-edit',
    data: null,
    closable: true
  })
}

// 处理状态变化事件
const handleStateChange = (tabType: 'action-list' | 'exploit-list', state: any) => {
  const tab = tabs.value.find(t => t.type === tabType)
  if (tab) {
    tab.pagination = state.pagination
    tab.search = state.search
    // 保存状态到 localStorage
    saveCurrentTabState()
  }
}

// 处理保存成功事件
const handleSaveSuccess = () => {
  // 不关闭编辑选项卡，保持当前编辑窗口打开
  // 不自动切换窗口，让用户继续在当前编辑页面工作
  
  // 可以选择性地刷新列表数据，但不切换窗口
  // 这样用户可以在编辑完成后手动切换到列表查看更新结果
}

// 导航到指定页面
const navigateTo = (type: 'action-list' | 'exploit-list') => {
  const existingTab = tabs.value.find(tab => tab.type === type)
  if (existingTab) {
    switchTab(existingTab.id)
  } else {
    let title = ''
    switch (type) {
      case 'action-list':
        title = '定时计划'
        break
      case 'exploit-list':
        title = '执行脚本'
        break
    }
    
    addTab({
      title,
      type,
      closable: false
    })
  }
}

// 检查是否有exploit-list选项卡
const hasExploitList = computed(() => tabs.value.some(tab => tab.type === 'exploit-list'))

// 检测屏幕大小
const checkScreenSize = () => {
  isMobile.value = window.innerWidth < 768
  if (isMobile.value) {
    isCollapsed.value = true
  }
}

// 切换折叠状态
const toggleCollapse = () => {
  isCollapsed.value = !isCollapsed.value
}

// 监听状态变化并自动保存
watch([tabs, activeTabId, isCollapsed], () => {
  saveState()
}, { deep: true })

// 监听窗口大小变化
onMounted(() => {
  // 先加载保存的状态
  loadState()
  
  checkScreenSize()
  window.addEventListener('resize', checkScreenSize)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkScreenSize)
})
</script>

<template>
  <div class="tab-manager">
    <!-- 左侧选项卡 -->
    <div class="tab-sidebar" :class="{ collapsed: isCollapsed }">
      <div class="sidebar-header">
        <el-button
          v-if="isMobile"
          @click="toggleCollapse"
          size="small"
          type="text"
          class="collapse-btn"
        >
          <el-icon><Menu /></el-icon>
        </el-button>
        <div v-if="!isCollapsed" class="header-content">
          <h1 class="sidebar-title">0E7工具箱</h1>
          <el-button 
            type="danger" 
            size="small" 
            @click="closeAllTabs"
            class="close-all-btn"
          >
            <el-icon><Close /></el-icon>
            关闭所有
          </el-button>
        </div>
      </div>
      
      <div v-if="!isCollapsed" class="tab-list">
        <div
          v-for="tab in tabs"
          :key="tab.id"
          :class="['tab-item', { 
            active: activeTabId === tab.id,
            closable: tab.closable
          }]"
          :data-title="tab.title"
          @click="switchTab(tab.id)"
        >
          <span class="tab-title">{{ tab.title }}</span>
          <el-icon
            v-if="tab.closable"
            class="tab-close"
            @click.stop="closeTab(tab.id)"
          >
            <Close />
          </el-icon>
        </div>
      </div>
      
    </div>
    
    <!-- 选项卡内容 -->
    <div class="tab-content">
      <div v-if="activeTab?.type === 'action-list'">
        <ActionList
          :pagination-state="activeTab.pagination"
          :search-state="activeTab.search"
          @edit="handleActionEdit"
          @add="handleActionAdd"
          @state-change="(state) => handleStateChange('action-list', state)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'exploit-list'">
        <ExploitList
          :pagination-state="activeTab.pagination"
          :search-state="activeTab.search"
          @edit="handleExploitEdit"
          @add="handleExploitAdd"
          @state-change="(state) => handleStateChange('exploit-list', state)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'action-edit'">
        <ActionEdit
          :action="activeTab.data"
          :is-editing="!!activeTab.data"
          :standalone="true"
          @save-success="handleSaveSuccess"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'exploit-edit'">
        <ExploitEdit
          :exploit="activeTab.data"
          :is-editing="!!activeTab.data"
          :standalone="true"
          @save-success="handleSaveSuccess"
        />
      </div>
      
    </div>
  </div>
</template>

<style scoped>
.tab-manager {
  height: 100vh;
  width: 100vw;
  display: flex;
  flex-direction: row;
  margin: 0;
  padding: 0;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.tab-sidebar {
  width: 180px;
  background: #fff;
  border-right: 1px solid #e6e8eb;
  display: flex;
  flex-direction: column;
  box-shadow: 2px 0 4px rgba(0, 0, 0, 0.1);
  transition: width 0.3s ease;
}

.tab-sidebar.collapsed {
  width: 50px;
}

.sidebar-header {
  padding: 12px 15px;
  border-bottom: 1px solid #e6e8eb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #f5f7fa;
}

.header-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.sidebar-title {
  font-weight: 600;
  color: #303133;
  font-size: 16px;
  margin: 0;
  text-align: center;
}

.close-all-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  width: 100%;
}

.collapse-btn {
  padding: 4px 8px;
}

.tab-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.tab-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #e6e8eb;
  cursor: pointer;
  background: #f5f7fa;
  color: #606266;
  transition: all 0.3s;
  position: relative;
  width: 100%;
  box-sizing: border-box;
}

.tab-item:not(.closable) {
  background: #e6f7ff;
  color: #1890ff;
  border-bottom: 2px solid #1890ff;
  font-weight: 600;
}

.tab-item:not(.closable).active {
  background: #1890ff;
  color: #fff;
  border-bottom: 2px solid #1890ff;
}

.tab-item:hover {
  background: #ecf5ff;
  color: #409eff;
}

.tab-item:not(.closable):hover {
  background: #bae7ff;
  color: #1890ff;
}

.tab-item.active {
  background: #409eff;
  color: #fff;
  border-bottom: 2px solid #409eff;
}

.tab-title {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-right: 8px;
  min-width: 0;
  position: relative;
}

.tab-item {
  position: relative;
}

.tab-item:hover::after {
  content: attr(data-title);
  position: absolute;
  left: 12px;
  top: 100%;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  white-space: nowrap;
  z-index: 1000;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  max-width: 300px;
  word-break: break-all;
  white-space: normal;
  margin-top: 4px;
}

.tab-close {
  font-size: 12px;
  opacity: 0.7;
  transition: opacity 0.3s;
}

.tab-close:hover {
  opacity: 1;
}


.tab-content {
  flex: 1;
  overflow: auto;
  padding: 20px;
  background: #f5f7fa;
  min-width: 0; /* 防止内容溢出 */
  height: 100vh;
  box-sizing: border-box;
  width: 100%;
}

.tab-content > div {
  width: 100%;
  height: 100%;
}


/* 滚动条样式 */
.tab-list::-webkit-scrollbar {
  width: 4px;
}

.tab-list::-webkit-scrollbar-track {
  background: #f1f1f1;
}

.tab-list::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 2px;
}

.tab-list::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .tab-sidebar {
    width: 50px;
  }
  
  .tab-sidebar:not(.collapsed) {
    width: 180px;
    position: fixed;
    top: 0;
    left: 0;
    height: 100vh;
    z-index: 1000;
    box-shadow: 2px 0 8px rgba(0, 0, 0, 0.15);
  }
  
  .tab-content {
    margin-left: 50px;
  }
  
  .tab-manager:has(.tab-sidebar:not(.collapsed)) .tab-content {
    margin-left: 0;
  }
}
</style>
