/**
 * 应用入口。
 * - Element Plus 全量引入（不做按需），统一中文语言包
 * - 路由只是为了后续可扩展（工具栏页、字典维护页等），当前只有一个主页面
 * - 使用 hash 模式，部署到后端静态目录时无需额外的服务端 rewrite 配置
 */
import { createApp } from 'vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

import 'element-plus/dist/index.css'
import './styles.css'

import App from './App.vue'
import MainView from './views/MainView.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'main', component: MainView, meta: { title: '加工程序管理' } },
    { path: '/:pathMatch(.*)*', redirect: '/' }
  ]
})

router.afterEach((to) => {
  const title = typeof to.meta.title === 'string' ? to.meta.title : ''
  document.title = title ? `${title} - CNC 加工程序管理系统` : 'CNC 加工程序管理系统'
})

const app = createApp(App)
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.mount('#app')
