<script setup lang="ts">
import MainForm from './components/MainForm.vue'
import OutputTable from './components/OutputTable.vue'
</script>

<template>
  <header>
    <h1 style="text-align: center;margin-bottom: 20px;"></h1>
  </header>

  <main>
    <MainForm />
    <div class="results-container">
      <h3 style="margin-bottom: 15px; color: #606266;">执行结果</h3>
      <OutputTable />
    </div>
  </main>
</template>

<style scoped>
.results-container {
  border: 1px solid #e6e8eb;
  border-radius: 4px;
  padding: 20px;
  margin-top: 20px;
  background: #fff;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.border-card {
  border-radius: 2px;
  margin-top: 20px;
}
</style>
