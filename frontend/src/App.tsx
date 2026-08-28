import { Routes, Route, Navigate } from 'react-router'
import AppLayout from './components/AppLayout'
import ErrorBoundary from './components/ErrorBoundary'
import Dashboard from './views/Dashboard'
import AddMonitor from './views/AddMonitor'
import MonitorDetail from './views/MonitorDetail'
import PushManagement from './views/PushManagement'
import PushHistory from './views/PushHistory'
import ScanRuleManagement from './views/ScanRuleManagement'
import RuleBuilder from './views/RuleBuilder'
import Settings from './views/Settings'

export default function App() {
  return (
    <ErrorBoundary>
      <AppLayout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/add" element={<AddMonitor />} />
        <Route path="/edit/:name" element={<AddMonitor />} />
        <Route path="/monitor/:name" element={<MonitorDetail />} />
        <Route path="/notifications" element={<PushManagement />} />
        <Route path="/push-logs" element={<PushHistory />} />
        <Route path="/rules" element={<ScanRuleManagement />} />
        <Route path="/rules/add" element={<RuleBuilder />} />
        <Route path="/settings" element={<Settings />} />
        {/* 旧路径重定向 */}
        <Route path="/push" element={<Navigate to="/notifications" replace />} />
        <Route path="/scan-rules" element={<Navigate to="/rules" replace />} />
      </Routes>
      </AppLayout>
    </ErrorBoundary>
  )
}
