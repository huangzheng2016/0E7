<script setup lang="ts">
import TabManager from './components/TabManager.vue'
</script>

<template>
  <div class="app-container">
    <TabManager />
  </div>
</template>

<style>
/* 全局重置样式 */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body {
  height: 100%;
  width: 100%;
  overflow: hidden;
}

#app {
  height: 100vh;
  width: 100vw;
}
</style>

<style scoped>
.app-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
  padding: 20px;
}

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
