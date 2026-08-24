import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import AddMonitor from '../views/AddMonitor.vue'
import MonitorDetail from '../views/MonitorDetail.vue'
import PushManagement from '../views/PushManagement.vue'
import ScanRuleManagement from '../views/ScanRuleManagement.vue'
import Settings from '../views/Settings.vue'

const routes = [
  { path: '/', name: 'dashboard', component: Dashboard },
  { path: '/add', name: 'add-monitor', component: AddMonitor },
  { path: '/edit/:name', name: 'edit-monitor', component: AddMonitor, props: true },
  { path: '/monitor/:name', name: 'monitor-detail', component: MonitorDetail, props: true },
  { path: '/push', name: 'push-management', component: PushManagement },
  { path: '/scan-rules', name: 'scan-rule-management', component: ScanRuleManagement },
  { path: '/settings', name: 'settings', component: Settings },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router