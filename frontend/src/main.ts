import "./assets/main.css";

import { createApp } from "vue";
// @ts-ignore
import { createStore } from "vuex";
// @ts-ignore
import App from "./App.vue";
import ElementPlus, { ElNotification } from "element-plus";
import "element-plus/dist/index.css";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";
import zhCn from 'element-plus/es/locale/lang/zh-cn';


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
      payload: { page?: number; pageSize?: number; exploit_id?: string } = {}
    ) {
      const { page = 1, pageSize = 20, exploit_id } = payload;
      // 如果没有传入exploit_id，则从URL参数获取
      const finalExploitId = exploit_id || new URLSearchParams(window.location.search).get('exploit_id');
      
      // 当exploit_id为空时，不查询output，直接返回空结果
      if (!finalExploitId) {
        state.workerQueue = [];
        state.totalItems = 0;
        return Promise.resolve({
          message: "success",
          total: 0,
          result: []
        });
      }
      
      const params = new URLSearchParams()
      params.append('exploit_id', finalExploitId.toString())
      params.append('page', page.toString())
      params.append('page_size', pageSize.toString())
      
      return fetch(`/webui/exploit_show_output?${params.toString()}`, {
        method: "GET"
      })
        .then((res) => res.json())
        .then((res) => {
          console.log('API响应数据:', res); // 调试信息
          if (res.result && Array.isArray(res.result)) {
            // 直接替换整个 workerQueue（只显示当前页的数据）
            state.workerQueue = res.result.map((item: any) => {
              return {
                id: item.id,
                exploit_id: item.exploit_id,
                client_id: item.client_id,
                client_name: item.client_name,
                status: item.status,
                output: item.output,
                update_time: item.update_time
              };
            });
          } else {
            // 当没有数据时，清空队列
            state.workerQueue = [];
          }
          
          // 使用后端返回的总条数
          if (res.total !== undefined) {
            state.totalItems = res.total;
          } else {
            state.totalItems = 0;
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
app.use(ElementPlus, {
  locale: zhCn,
});
app.mount("#app");
