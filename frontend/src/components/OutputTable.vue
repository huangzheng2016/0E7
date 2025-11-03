<script setup lang="ts">
import type { ElTable } from 'element-plus';
import { watch, ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useStore } from 'vuex';

// 接收名称作为prop
const props = defineProps<{
  exploitName?: string
}>()

type chunk = {
    id: number,
    output: string,
    status: string,
    update_time: string,
}

const store = useStore();
const data = ref<Array<chunk>>([]);

// 分页相关状态
const currentPage = ref(1);
const pageSize = ref(20);
const totalItems = ref(0);

// 内置自动刷新开关与逻辑（默认关闭）
const autoRefresh = ref(false);
const refreshIntervalSec = ref(5);
let refreshTimer: number | undefined;

const startAutoRefresh = () => {
    stopAutoRefresh();
    if (refreshIntervalSec.value <= 0) return;
    refreshTimer = window.setInterval(() => {
        refreshAllData();
    }, refreshIntervalSec.value * 1000);
}

const stopAutoRefresh = () => {
    if (refreshTimer !== undefined) {
        clearInterval(refreshTimer);
        refreshTimer = undefined;
    }
}

// 全量刷新函数
const refreshAllData = () => {
    const exploitId = props.exploitName || new URLSearchParams(window.location.search).get('exploit_id');
    store.dispatch('fetchResults', { 
        page: currentPage.value, 
        pageSize: pageSize.value,
        exploit_id: exploitId 
    }).then(() => {
        // 刷新完成后确保分页状态同步
        totalItems.value = store.state.totalItems;
    }).catch(error => {
        console.error('刷新数据失败:', error);
    });
}

// 格式化时间显示
const formatTime = (timeString?: string) => {
    if (!timeString) return '-';
    const date = new Date(timeString);
    return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    });
}

// 处理页码变化
const handlePageChange = (page: number) => {
    currentPage.value = page;
    refreshAllData();
}

// 处理每页数量变化
const handleSizeChange = (size: number) => {
    pageSize.value = size;
    currentPage.value = 1;
    refreshAllData();
}

// 取消内部自动刷新开关，改由父组件控制刷新节奏

// 监听名称变化，重新获取数据
watch(() => props.exploitName, () => {
    if (props.exploitName) {
        refreshAllData();
    }
}, { immediate: true });

onMounted(() => {
    // 初始加载数据
    refreshAllData();
    
    // 确保分页状态正确初始化
    nextTick(() => {
        // 如果 store 中已经有数据，确保分页状态同步
        if (store.state.totalItems > 0) {
            totalItems.value = store.state.totalItems;
        }
    });
    
    // 监听自定义刷新事件
    window.addEventListener('refresh-output', refreshAllData);

    // 根据开关状态启动自动刷新
    if (autoRefresh.value) {
        startAutoRefresh();
    }
});

// 组件卸载时清理定时器和事件监听器
onUnmounted(() => {
    window.removeEventListener('refresh-output', refreshAllData);
    stopAutoRefresh();
});

watch(() => store.state.workerQueue, (newVal) => {
    // 直接替换整个数据数组
    data.value = newVal.map((item: any) => ({
        ...item,
    }));
    
}, { deep: true });

watch (() => store.state.totalItems, (newVal) => {
    totalItems.value = newVal;
});

// 监听自动刷新开关
watch(autoRefresh, (enabled) => {
    if (enabled) {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
});

// 监听刷新间隔调整，若已开启自动刷新则重启计时器
watch(refreshIntervalSec, () => {
    if (autoRefresh.value) {
        startAutoRefresh();
    }
});
</script>

<template>
    <div class="table-container">
        <ElTable :height="425" :data="data">
            <ElTableColumn label="ID" prop="id" width="80"></ElTableColumn>
            <ElTableColumn label="状态" prop="status" width="100">
                <template #default="{ row }">
                    <el-tag 
                        :type="row.status === 'SUCCESS' ? 'success' : row.status === 'ERROR' ? 'danger' : row.status === 'TIMEOUT' ? 'warning' : 'info'"
                        size="small"
                    >
                        {{ row.status }}
                    </el-tag>
                </template>
            </ElTableColumn>
            <ElTableColumn label="执行客户端" prop="client_name" width="120">
                <template #default="{ row }">
                    <span v-if="row.client_name">{{ (row.client_name || '').slice(0, 8) }}</span>
                    <span v-else class="text-muted">未知</span>
                </template>
            </ElTableColumn>
            <ElTableColumn label="Team" prop="team" width="120">
                <template #default="{ row }">
                    <el-tag v-if="row.team" size="small">{{ row.team }}</el-tag>
                    <span v-else class="text-muted">未设置</span>
                </template>
            </ElTableColumn>
            <ElTableColumn label="更新时间" width="160">
                <template #default="{ row }">
                    {{ formatTime(row.update_time) }}
                </template>
            </ElTableColumn>
            <ElTableColumn label="输出结果" prop="output">
                <template #default="{ row }">
                    <pre class="output-pre">{{ row.output }}</pre>
                </template>
            </ElTableColumn>
        </ElTable>
        
        <!-- 底部控件和分页 -->
        <div class="bottom-controls">
            <!-- 刷新控件（左侧） -->
            <div class="refresh-control">
                <el-button 
                    type="primary" 
                    @click="refreshAllData"
                >
                    <el-icon><Refresh /></el-icon>
                    刷新
                </el-button>
                <el-switch v-model="autoRefresh" active-text="自动刷新" />
                <el-input-number v-model="refreshIntervalSec" :min="1" :max="300" />
            </div>
            
            <!-- 分页控件（右侧） -->
            <div class="pagination-control">
                <el-pagination
                    v-model:current-page="currentPage"
                    v-model:page-size="pageSize"
                    :page-sizes="[10, 20, 50, 100]"
                    size="small"
                    :background="true"
                    layout="total, sizes, prev, pager, next"
                    :total="totalItems"
                    :hide-on-single-page="false"
                    @current-change="handlePageChange"
                    @size-change="handleSizeChange"
                />
            </div>
        </div>
    </div>
</template>

<style scoped>
.table-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.bottom-controls {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 20px;
    padding-top: 15px;
    border-top: 1px solid #e6e8eb;
    flex-shrink: 0;
}

.refresh-control {
    display: flex;
    align-items: center;
    gap: 12px;
}

.pagination-control {
    display: flex;
    justify-content: flex-end;
}

.output-pre {
    margin: 0;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 100px;
    overflow-y: auto;
    background: #f5f7fa;
    padding: 8px;
    border-radius: 4px;
}

.el-table :deep(.cell) {
    line-height: 1.4;
}

.text-muted {
    color: #909399;
    font-style: italic;
}
</style>