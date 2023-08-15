<script setup lang="ts">
import MainForm from './components/MainForm.vue'
import OutputPanel from './components/OutputPanel.vue'
import OutputTable from './components/OutputTable.vue'
</script>

<template>
  <header>
    <h1 style="text-align: center;margin-bottom: 40px;">远程执行工具</h1>
  </header>

  <main>
    <MainForm />
    <el-tabs type="border-card" class="border-card">
      <el-tab-pane label="命令行"><OutputPanel /></el-tab-pane>
      <el-tab-pane label="表格"><OutputTable /></el-tab-pane>
    </el-tabs>
  </main>
</template>


<style scoped>
.border-card {
  border-radius: 2px;
  margin-top: 20px;
}
</style>
