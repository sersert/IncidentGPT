import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/Layout.tsx'
import { Dashboard } from '@/pages/Dashboard.tsx'
import { IncidentList } from '@/pages/IncidentList.tsx'
import { IncidentDetail } from '@/pages/IncidentDetail.tsx'
import { AlertFeed } from '@/pages/AlertFeed.tsx'
import { Analytics } from '@/pages/Analytics.tsx'
import { Predictions } from '@/pages/Predictions.tsx'
import { Settings } from '@/pages/Settings.tsx'
import { NotFound } from '@/pages/NotFound.tsx'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/incidents" element={<IncidentList />} />
          <Route path="/incidents/:id" element={<IncidentDetail />} />
          <Route path="/alerts" element={<AlertFeed />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="/predictions" element={<Predictions />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/settings/:section" element={<Settings />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
