<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import ActionList from './ActionList.vue'
import ExploitList from './ExploitList.vue'
import PcapList from './PcapList.vue'
import ActionEdit from './ActionEdit.vue'
import ExploitEdit from './ExploitEdit.vue'
import PcapDetail from './PcapDetail.vue'
import FlagList from './FlagList.vue'
import TerminalManagement from './TerminalManagement.vue'
import ProxyCache from './ProxyCache.vue'
import GitRepoList from './GitRepoList.vue'

interface Tab {
  id: string
  title: string
  type: 'action-list' | 'exploit-list' | 'pcap-list' | 'flag-list' | 'terminal-management' | 'proxy-cache' | 'git-repo-list' | 'action-edit' | 'exploit-edit' | 'pcap-detail'
  // 只保存ID，不保存完整数据
  itemId?: number | string  // action的id、exploit的id或pcap的id
  closable: boolean
  // 搜索相关状态
  searchState?: {
    src_ip?: string
    dst_ip?: string
    tags?: string
    fulltext?: string
    flagInActive?: boolean
    flagOutActive?: boolean
    searchType?: number
  }
  // 分页相关状态
  paginationState?: {
    currentPage: number
    pageSize: number
    totalItems: number
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
  },
  {
    id: 'pcap-list',
    title: '流量分析',
    type: 'pcap-list',
    closable: false
  },
  {
    id: 'flag-list',
    title: 'Flag管理',
    type: 'flag-list',
    closable: false
  },
  {
    id: 'terminal-management',
    title: '终端管理',
    type: 'terminal-management',
    closable: false
  },
  {
    id: 'proxy-cache',
    title: '代理缓存',
    type: 'proxy-cache',
    closable: false
  },
  {
    id: 'git-repo-list',
    title: 'Git 仓库',
    type: 'git-repo-list',
    closable: false
  }
])

const activeTabId = ref('action-list')
const isCollapsed = ref(false)
const isMobile = ref(false)

// 标签页持久化相关 - 只保存基本信息
const STORAGE_KEY = 'tab-manager-state'

// 保存状态到 localStorage
const saveState = () => {
  const state = {
    tabs: tabs.value.map(tab => ({
      id: tab.id,
      title: tab.title,
      type: tab.type,
      itemId: tab.itemId,
      closable: tab.closable,
      searchState: tab.searchState,
      paginationState: tab.paginationState
    })),
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
      
      // 恢复标签页基本信息，但保留默认的基础标签页
      if (state.tabs && Array.isArray(state.tabs)) {
        // 确保基础标签页存在
        const defaultTabs: Tab[] = [
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
          },
          {
            id: 'pcap-list',
            title: '流量分析',
            type: 'pcap-list',
            closable: false
          },
          {
            id: 'flag-list',
            title: 'Flag管理',
            type: 'flag-list',
            closable: false
          },
          {
            id: 'terminal-management',
            title: '终端管理',
            type: 'terminal-management',
            closable: false
          },
          {
            id: 'proxy-cache',
            title: '代理缓存',
            type: 'proxy-cache',
            closable: false
          },
          {
            id: 'git-repo-list',
            title: 'Git 仓库',
            type: 'git-repo-list',
            closable: false
          }
        ]
        
        // 合并默认标签页和保存的标签页
        const savedTabs = state.tabs || []
        const mergedTabs = [...defaultTabs]
        
        // 恢复基础标签页的状态
        savedTabs.forEach((savedTab: any) => {
          const defaultTab = mergedTabs.find(tab => tab.type === savedTab.type)
          if (defaultTab) {
            // 恢复基础标签页的状态
            defaultTab.searchState = savedTab.searchState
            defaultTab.paginationState = savedTab.paginationState
          } else if (savedTab.type === 'action-edit' || savedTab.type === 'exploit-edit' || savedTab.type === 'pcap-detail') {
            // 添加保存的编辑标签页
            mergedTabs.push(savedTab as Tab)
          }
        })
        
        tabs.value = mergedTabs
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
  if (index > -1 && tabs.value[index]?.closable) {
    const closedTab = tabs.value[index]
    if (!closedTab) return
    
    tabs.value.splice(index, 1)
    
    // 如果关闭的是当前活动选项卡，切换到其他选项卡
    if (activeTabId.value === tabId) {
      if (tabs.value.length > 0) {
        const nextTab = tabs.value[Math.max(0, index - 1)]
        if (nextTab) {
          activeTabId.value = nextTab.id
        }
      }
      
      // 清理URL参数
      const url = new URL(window.location.href)
      if (closedTab.type === 'action-edit') {
        url.searchParams.delete('action_id')
      } else if (closedTab.type === 'exploit-edit') {
        url.searchParams.delete('exploit_id')
      } else if (closedTab.type === 'pcap-detail') {
        url.searchParams.delete('pcap_id')
        url.searchParams.delete('search')
      }
      window.history.pushState({ path: url.href }, '', url.href)
    }
  }
}

// 关闭所有标签页
const closeAllTabs = () => {
  // 只保留不可关闭的标签页（通常是列表页面）
  tabs.value = tabs.value.filter(tab => !tab.closable)
  
  // 切换到第一个标签页
  if (tabs.value.length > 0 && tabs.value[0]) {
    activeTabId.value = tabs.value[0].id
  }
  
  // 清理URL参数，回到干净的状态
  const url = new URL(window.location.href)
  url.searchParams.delete('action_id')
  url.searchParams.delete('exploit_uuid')
  url.searchParams.delete('pcap_id')
  url.searchParams.delete('search')
  window.history.pushState({ path: url.href }, '', url.href)
}

// 暴露方法给父组件
defineExpose({
  closeAllTabs
})

// 切换到指定选项卡
const switchTab = (tabId: string) => {
  // 直接切换标签页，不需要保存/恢复状态
  activeTabId.value = tabId
  
  // 更新URL以反映当前活动的编辑标签页
  updateUrlForActiveTab()
}

// 根据当前活动标签页更新URL
const updateUrlForActiveTab = () => {
  const activeTab = tabs.value.find(tab => tab.id === activeTabId.value)
  if (!activeTab) return
  
  const url = new URL(window.location.href)
  
      // 清除所有编辑相关的参数
      url.searchParams.delete('action_id')
      url.searchParams.delete('exploit_id')
      url.searchParams.delete('name')
      url.searchParams.delete('pcap_id')
      url.searchParams.delete('search')
      
      // 根据标签页类型设置相应的参数
      if (activeTab.type === 'action-edit') {
        if (activeTab.itemId) {
          url.searchParams.set('action_id', activeTab.itemId.toString())
        } else {
          url.searchParams.set('action_id', 'new')
        }
      } else if (activeTab.type === 'exploit-edit') {
        if (activeTab.itemId) {
          // 对于exploit，使用name参数而不是exploit_id
          url.searchParams.set('name', activeTab.itemId.toString())
        } else {
          url.searchParams.set('name', 'new')
        }
      } else if (activeTab.type === 'pcap-detail') {
        if (activeTab.itemId) {
          url.searchParams.set('pcap_id', activeTab.itemId.toString())
        }
        if (activeTab.searchState?.fulltext) {
          url.searchParams.set('search', activeTab.searchState.fulltext)
        }
      } else if (activeTab.type === 'pcap-list' && activeTab.searchState?.fulltext) {
        url.searchParams.set('search', activeTab.searchState.fulltext)
      }
  
  window.history.pushState({ path: url.href }, '', url.href)
}

// 简化的状态管理 - 不再保存分页和搜索状态
// 页面刷新时会重新从服务器获取数据

// 获取当前活动选项卡
const activeTab = computed(() => {
  return tabs.value.find(tab => tab.id === activeTabId.value)
})

// 处理ActionList的事件
const handleActionEdit = (action: any) => {
  // 检查是否已存在相同ID的Action编辑标签页
  const existingTab = tabs.value.find(tab => tab.type === 'action-edit' && tab.itemId === action.id)
  
  if (existingTab) {
    // 如果存在相同ID的标签页，直接切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，创建新标签页
    addTab({
      title: `${action.name} - 编辑定时计划`,
      type: 'action-edit',
      itemId: action.id, // 只保存ID，不保存完整数据
      closable: true
    })
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

const handleActionAdd = () => {
  // 检查是否已存在新增Action编辑标签页（itemId为undefined）
  const existingTab = tabs.value.find(tab => tab.type === 'action-edit' && tab.itemId === undefined)
  
  if (existingTab) {
    // 如果存在新增标签页，直接切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，创建新标签页
    addTab({
      title: '新增定时计划',
      type: 'action-edit',
      itemId: undefined, // 新增时没有ID
      closable: true
    })
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

// 处理ExploitList的事件
const handleExploitEdit = (exploit: any) => {
  // 检查是否已存在相同ID的Exploit编辑标签页
  const existingTab = tabs.value.find(tab => tab.type === 'exploit-edit' && tab.itemId === exploit.id)
  
  if (existingTab) {
    // 如果存在相同ID的标签页，直接切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，创建新标签页
    addTab({
      title: `${exploit.name} - 编辑执行脚本`,
      type: 'exploit-edit',
      itemId: exploit.id, // 只保存ID，不保存完整数据
      closable: true
    })
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

// 通过ID处理Exploit编辑（从FlagList调用）
const handleExploitEditById = async (exploitId: number) => {
  // 检查是否已存在相同ID的Exploit编辑标签页
  const existingTab = tabs.value.find(tab => tab.type === 'exploit-edit' && tab.itemId === exploitId)
  
  if (existingTab) {
    // 如果存在相同ID的标签页，直接切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，需要先获取exploit信息，然后创建新标签页
    try {
      // 这里可以调用API获取exploit信息，或者直接使用ID创建标签页
      // 为了简化，我们直接使用ID创建标签页，ExploitEdit组件会自己获取数据
      addTab({
        title: `执行脚本 #${exploitId} - 编辑`,
        type: 'exploit-edit',
        itemId: exploitId,
        closable: true
      })
    } catch (error) {
      console.error('获取exploit信息失败:', error)
    }
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

// 处理PcapList的事件
const handlePcapView = (pcap: any) => {
  // 检查是否已存在相同ID的Pcap详情标签页
  const existingTab = tabs.value.find(tab => tab.type === 'pcap-detail' && tab.itemId === pcap.id)
  
  if (existingTab) {
    // 如果存在相同ID的标签页，切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，创建新标签页
    addTab({
      title: `流量详情 -  ${pcap.id}`,
      type: 'pcap-detail',
      itemId: pcap.id, // 只保存ID，不保存完整数据
      closable: true,
      // 传递搜索关键字
      searchState: pcap._searchKeyword ? { fulltext: pcap._searchKeyword } : undefined
    })
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

const handleExploitAdd = () => {
  // 检查是否已存在新增Exploit编辑标签页（itemId为undefined）
  const existingTab = tabs.value.find(tab => tab.type === 'exploit-edit' && tab.itemId === undefined)
  
  if (existingTab) {
    // 如果存在新增标签页，直接切换到它
    activeTabId.value = existingTab.id
  } else {
    // 如果不存在，创建新标签页
    addTab({
      title: '新增执行脚本',
      type: 'exploit-edit',
      itemId: undefined, // 新增时没有ID
      closable: true
    })
  }
  
  // 更新URL以便分享
  updateUrlForActiveTab()
}

// 简化的状态变化处理 - 保存搜索状态
const handleStateChange = (tabType: 'action-list' | 'exploit-list' | 'pcap-list' | 'flag-list', state: any) => {
  const tab = tabs.value.find(t => t.type === tabType)
  if (tab) {
    // 保存搜索状态
    if (state.search) {
      tab.searchState = state.search
    }
    // 保存分页状态
    if (state.pagination) {
      tab.paginationState = state.pagination
    }
  }
}

// 处理保存成功事件
const handleSaveSuccess = () => {
  // 不关闭编辑选项卡，保持当前编辑窗口打开
  // 不自动切换窗口，让用户继续在当前编辑页面工作
  
  // 触发输出页面的刷新
  window.dispatchEvent(new CustomEvent('refresh-output'))
  
  // 可以选择性地刷新列表数据，但不切换窗口
  // 这样用户可以在编辑完成后手动切换到列表查看更新结果
}

// 导航到指定页面
const navigateTo = (type: 'action-list' | 'exploit-list' | 'pcap-list' | 'flag-list' | 'terminal-management') => {
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
      case 'pcap-list':
        title = '流量分析'
        break
      case 'flag-list':
        title = 'Flag管理'
        break
      case 'terminal-management':
        title = '终端管理'
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
  const wasMobile = isMobile.value
  isMobile.value = window.innerWidth < 768
  
  // 如果从移动端切换到桌面端，自动展开侧边栏
  if (wasMobile && !isMobile.value) {
    isCollapsed.value = false
  }
  // 如果从桌面端切换到移动端，自动折叠侧边栏
  else if (!wasMobile && isMobile.value) {
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

// 根据URL参数自动打开对应的编辑标签页
const openTabFromUrl = () => {
  const urlParams = new URLSearchParams(window.location.search)
  const actionId = urlParams.get('action_id')
  const exploitId = urlParams.get('exploit_id')
  const pcapId = urlParams.get('pcap_id')
  const searchKeyword = urlParams.get('search')
  
  if (actionId) {
    if (actionId === 'new') {
      // 检查是否已存在新增Action编辑标签页
      const existingTab = tabs.value.find(tab => tab.type === 'action-edit' && tab.itemId === undefined)
      
      if (existingTab) {
        // 如果存在新增标签页，直接切换到它
        activeTabId.value = existingTab.id
      } else {
        // 如果不存在，创建新标签页
        addTab({
          title: '新增定时计划',
          type: 'action-edit',
          itemId: undefined,
          closable: true
        })
      }
    } else {
      // 检查是否已存在相同ID的Action编辑标签页
      const existingTab = tabs.value.find(tab => tab.type === 'action-edit' && tab.itemId === parseInt(actionId))
      
      if (existingTab) {
        // 如果存在相同ID的标签页，直接切换到它
        activeTabId.value = existingTab.id
      } else {
        // 如果不存在，创建新标签页
        addTab({
          title: `${actionId} - 编辑定时计划`,
          type: 'action-edit',
          itemId: parseInt(actionId),
          closable: true
        })
      }
    }
  } else if (exploitId) {
    if (exploitId === 'new') {
      // 检查是否已存在新增Exploit编辑标签页
      const existingTab = tabs.value.find(tab => tab.type === 'exploit-edit' && tab.itemId === undefined)
      
      if (existingTab) {
        // 如果存在新增标签页，直接切换到它
        activeTabId.value = existingTab.id
      } else {
        // 如果不存在，创建新标签页
        addTab({
          title: '新增执行脚本',
          type: 'exploit-edit',
          itemId: undefined,
          closable: true
        })
      }
    } else {
      // 检查是否已存在相同ID的Exploit编辑标签页
      const existingTab = tabs.value.find(tab => tab.type === 'exploit-edit' && tab.itemId === parseInt(exploitId))
      
      if (existingTab) {
        // 如果存在相同ID的标签页，直接切换到它
        activeTabId.value = existingTab.id
      } else {
        // 如果不存在，创建新标签页
        addTab({
          title: `${exploitId} - 编辑执行脚本`,
          type: 'exploit-edit',
          itemId: parseInt(exploitId),
          closable: true
        })
      }
    }
  } else if (pcapId) {
    // 检查是否已存在Pcap详情标签页
    const existingTab = tabs.value.find(tab => tab.type === 'pcap-detail')
    
    if (existingTab) {
      // 如果存在，更新现有标签页并切换到它
      existingTab.title = `流量详情 -  ${pcapId}`
      existingTab.itemId = parseInt(pcapId)
      if (searchKeyword) {
        existingTab.searchState = { fulltext: searchKeyword }
      }
      activeTabId.value = existingTab.id
    } else {
      // 如果不存在，创建新标签页
      addTab({
        title: `流量详情 -  ${pcapId}`,
        type: 'pcap-detail',
        itemId: parseInt(pcapId),
        closable: true,
        searchState: searchKeyword ? { fulltext: searchKeyword } : undefined
      })
    }
  }
  
  // 如果有搜索关键字但没有pcap_id，切换到pcap-list并设置搜索状态
  if (searchKeyword && !pcapId) {
    const pcapListTab = tabs.value.find(tab => tab.type === 'pcap-list')
    if (pcapListTab) {
      pcapListTab.searchState = { fulltext: searchKeyword }
      activeTabId.value = pcapListTab.id
    }
  }
}

// 监听窗口大小变化
onMounted(() => {
  // 先加载保存的状态
  loadState()
  
  // 根据URL参数自动打开对应的编辑标签页
  openTabFromUrl()
  
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
          @edit="handleActionEdit"
          @add="handleActionAdd"
          @state-change="(state: any) => handleStateChange('action-list', state)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'exploit-list'">
        <ExploitList
          @edit="handleExploitEdit"
          @add="handleExploitAdd"
          @state-change="(state: any) => handleStateChange('exploit-list', state)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'pcap-list'">
        <PcapList
          :search-state="activeTab.searchState"
          :pagination-state="activeTab.paginationState"
          @view="handlePcapView"
          @state-change="(state: any) => handleStateChange('pcap-list', state)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'flag-list'">
        <FlagList
          @state-change="(state: any) => handleStateChange('flag-list', state)"
          @open-exploit-edit="handleExploitEditById"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'terminal-management'">
        <TerminalManagement />
      </div>
      
      <div v-else-if="activeTab?.type === 'proxy-cache'">
        <ProxyCache />
      </div>
      
      <div v-else-if="activeTab?.type === 'git-repo-list'">
        <GitRepoList />
      </div>
      
      <div v-else-if="activeTab?.type === 'action-edit'">
        <ActionEdit
          :action-id="activeTab.itemId"
          :is-editing="!!activeTab.itemId"
          :standalone="true"
          @save-success="handleSaveSuccess"
          @close="closeTab(activeTab.id)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'exploit-edit'">
        <ExploitEdit
          :exploit-id="activeTab.itemId?.toString()"
          :is-editing="!!activeTab.itemId"
          :standalone="true"
          @save-success="handleSaveSuccess"
          @close="closeTab(activeTab.id)"
        />
      </div>
      
      <div v-else-if="activeTab?.type === 'pcap-detail'">
        <PcapDetail
          :pcap-id="activeTab.itemId as number"
          :search-keyword="activeTab.searchState?.fulltext"
          @close="closeTab(activeTab.id)"
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
    width: 180px !important;
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

/* 确保桌面端侧边栏正常显示 */
@media (min-width: 769px) {
  .tab-sidebar {
    width: 180px !important;
  }
  
  .tab-sidebar.collapsed {
    width: 50px !important;
  }
}
</style>
