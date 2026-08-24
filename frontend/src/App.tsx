import { Routes, Route } from 'react-router'
import AppLayout from './components/AppLayout'
import ErrorBoundary from './components/ErrorBoundary'
import Dashboard from './views/Dashboard'
import AddMonitor from './views/AddMonitor'
import MonitorDetail from './views/MonitorDetail'
import PushManagement from './views/PushManagement'
import ScanRuleManagement from './views/ScanRuleManagement'
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
        <Route path="/push" element={<PushManagement />} />
        <Route path="/scan-rules" element={<ScanRuleManagement />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
      </AppLayout>
    </ErrorBoundary>
  )
}
