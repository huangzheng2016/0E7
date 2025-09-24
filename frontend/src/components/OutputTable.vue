<script setup lang="ts">
import type { ElTable } from 'element-plus';
import { watch, ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useStore } from 'vuex';

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

// 自动刷新开关
const autoRefresh = ref(false);

// 全量刷新函数
const refreshAllData = () => {
    store.dispatch('fetchResults', { page: currentPage.value, pageSize: pageSize.value }).catch(error => {
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

// 设置定时器，每3秒全量刷新一次
const refreshInterval = ref<number>();

// 启动自动刷新
const startAutoRefresh = () => {
    if (refreshInterval.value) {
        clearInterval(refreshInterval.value);
    }
    refreshInterval.value = setInterval(() => {
        refreshAllData();
    }, 3000);
}

// 停止自动刷新
const stopAutoRefresh = () => {
    if (refreshInterval.value) {
        clearInterval(refreshInterval.value);
        refreshInterval.value = undefined;
    }
}

// 监听自动刷新开关变化
watch(autoRefresh, (newValue) => {
    if (newValue) {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
});

onMounted(() => {
    // 初始加载数据
    refreshAllData();
});

// 组件卸载时清理定时器
onUnmounted(() => {
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
</script>

<template>
    <div class="table-container">
        <ElTable :height="425" :data="data">
            <ElTableColumn label="ID" prop="id" width="80"></ElTableColumn>
            <ElTableColumn label="状态" prop="status" width="100">
                <template #default="{ row }">
                    <el-tag 
                        :type="row.status === 'SUCCESS' ? 'success' : row.status === 'ERROR' ? 'danger' : 'warning'"
                        size="small"
                    >
                        {{ row.status }}
                    </el-tag>
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
        
        <!-- 底部控件 -->
        <div class="bottom-controls">
            <!-- 自动刷新控件（左下角） -->
            <div class="refresh-control">
                <el-checkbox v-model="autoRefresh">自动刷新</el-checkbox>
            </div>
            
            <!-- 分页控件（右下角） -->
            <div class="pagination-control">
                <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :small="true"
                :background="true"
                layout="total, sizes, prev, pager, next"
                :total="totalItems"
                :hide-on-single-page="false"
                @current-change="handlePageChange"
                @size-change="handleSizeChange"
            >
                <template #total="{ total }">
                    总计 {{ total }} 条
                </template>
                <template #sizes="{ sizes }">
                    <span class="el-pagination__sizes">
                        <span class="el-pagination__sizes-text">每页</span>
                        <el-select v-model="pageSize" @change="handleSizeChange" size="small">
                            <el-option
                                v-for="size in sizes"
                                :key="size"
                                :label="size + ' 条'"
                                :value="size"
                            />
                        </el-select>
                    </span>
                </template>
            </el-pagination>
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
    margin-top: 16px;
}

.refresh-control {
    display: flex;
    align-items: center;
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
</style>