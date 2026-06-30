import { Routes, Route } from 'react-router-dom'
import HomePage from './pages/HomePage'
import PastePage from './pages/PastePage'
import MyPastesPage from './pages/MyPastesPage'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/my-pastes" element={<MyPastesPage />} />
      <Route path="/:key" element={<PastePage />} />
    </Routes>
  )
}

