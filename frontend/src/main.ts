import "./assets/main.css";

import { createApp } from "vue";
// @ts-ignore
import { createStore } from "vuex";
// @ts-ignore
import App from "./App.vue";
import ElementPlus, { ElNotification } from "element-plus";
import "element-plus/dist/index.css";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";


const store = createStore({
  state() {
    return {
      workerQueue: [],
      totalItems: 0
    };
  },
  mutations: {},
  actions: {
    fetchResults( 
      {
        state
      }: {
        state: any;
      },
      payload: { page?: number; pageSize?: number } = {}
    ) {
      const { page = 1, pageSize = 20 } = payload;
      const urlParams = new URLSearchParams(window.location.search);
      const uuid = urlParams.get('uuid');
      
      let requestBody = '';
      if (uuid) {
        requestBody = `exploit_uuid=${uuid}`;
      }
      // 添加分页参数
      requestBody += `&page=${page}&page_size=${pageSize}`;
      
      return fetch("/webui/exploit_show_output", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: requestBody
      })
        .then((res) => res.json())
        .then((res) => {
          if (res.result && Array.isArray(res.result)) {
            // 直接替换整个 workerQueue（只显示当前页的数据）
            state.workerQueue = res.result.map((item: any) => ({
              id: item.id,
              status: item.status,
              output: item.output,
              update_time: item.update_time
            }));
            
            // 使用后端返回的总条数
            if (res.total !== undefined) {
              state.totalItems = res.total;
            }
          }
          return res;
        })
        .catch((error) => {
          console.error("获取结果失败:", error);
          throw error;
        });
    }
  },
  getters: {},
});

const app = createApp(App);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}
app.use(store);
app.use(ElementPlus);
app.mount("#app");
