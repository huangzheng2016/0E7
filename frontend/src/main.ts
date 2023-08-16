import "./assets/main.css";

import { createApp } from "vue";
import { createStore } from "vuex";
import App from "./App.vue";
import ElementPlus, { ElNotification } from "element-plus";
import "element-plus/dist/index.css";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";


const store = createStore({
  state() {
    return {
      workerQueue: [],
    };
  },
  mutations: {
    push(
      state: {
        workerQueue: Array<{
          uuid: string;
          status: string;
          content: string;
        }>;
      },
      payload: string
    ) {
      state.workerQueue = [
        {
          status: "Pending",
          uuid: payload,
          content: "",
        },
        ...state.workerQueue,
      ];
    },
    change(
      state: {
        workerQueue: Array<{
          uuid: string;
          status: string;
          content: string;
        }>;
      },
      payload: { uuid: string; content: string; STATUS: string }
    ) {
      const index = state.workerQueue.findIndex(
        (item) => item.uuid === payload.uuid
      );
      if (index !== -1) {
        state.workerQueue[index].content = payload.content;
        state.workerQueue[index].status = payload.STATUS;
        let tmp = state.workerQueue[index];
        state.workerQueue.splice(index, 1);
        state.workerQueue.unshift(tmp);
      }
    },
  },
  actions: {
    change(
      {
        commit,
      }: {
        commit: (
          arg0: string,
          arg1: { uuid: string; content: string; STATUS: string }
        ) => void;
      },
      payload: { uuid: string; content: string; STATUS: string; intv?: number }
    ) {
      fetch("/api/list", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: `exploit_uuid=${payload.uuid}`,
      })
        .then((res) => res.json(), (err) => {
          ElNotification({
            title: "与目标主机失去连接，请重试",
            message: err,
            type: "error",
          });
          clearInterval(payload.intv);
        })
        .then((res) => {
          if (res.result !== null) {
            res = res.result[0];
            payload.content = res.output;
            payload.STATUS = res.status;
            commit("change", payload);
            clearInterval(payload.intv);
          }
        });
    },
  },
  getters: {
    top(state: {
      workerQueue: Array<{
        uuid: string;
        status: string;
        content: string;
      }>;
    }) {
      return state.workerQueue[state.workerQueue.length - 1];
    },
  },
});

const app = createApp(App);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}
app.use(store);
app.use(ElementPlus);
app.mount("#app");
