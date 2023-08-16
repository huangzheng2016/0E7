<script setup lang="ts">
import type { ElTable } from 'element-plus';
import { watch, ref, computed } from 'vue';
import { useStore } from 'vuex';

type chunk = {
    uuid: string,
    output: string,
    status: string,
    intv?: number
}

const store = useStore();
const data = ref<Array<chunk>>([]);
const asyncChange = (row: chunk) => {
    store.dispatch('change', row);
}

const tableData = computed(() => store.state.workerQueue);

watch(() => store.state.workerQueue, (newVal) => {
    newVal.forEach((item: chunk) => {
        data.value.push({
            uuid: item.uuid,
            output: item.output,
            status: item.status
        });
        if (item.status === 'Pending') {
            let intv = setInterval(() => {
                item.intv = intv;
                asyncChange(item);
            }, 1000);
        }
    });
});

</script>

<template>
    <ElTable :height="425" :data="tableData">
        <ElTableColumn label="UUID" prop="uuid"></ElTableColumn>
        <ElTableColumn label="Status" prop="status"></ElTableColumn>
        <ElTableColumn label="Output" prop="content"></ElTableColumn>
    </ElTable>
</template>