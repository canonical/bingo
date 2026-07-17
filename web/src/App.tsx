import { Routes, Route } from 'react-router-dom'
import AuthGuard from './components/AuthGuard/AuthGuard'
import HomePage from './pages/HomePage'
import PastePage from './pages/PastePage'
import MyPastesPage from './pages/MyPastesPage'

export default function App() {
  return (
    <AuthGuard>
      <div role="presentation">
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/my-pastes" element={<MyPastesPage />} />
          <Route path="/:key" element={<PastePage />} />
        </Routes>
      </div>
    </AuthGuard>
  )
}

