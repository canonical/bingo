import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import Navigation from '../components/Navigation/Navigation'
import MyPastesList from '../components/MyPastesList/MyPastesList'

export default function MyPastesPage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  if (!isAuthenticated) return null

  return (
    <>
      <Navigation isAuthenticated userEmail={undefined} />
      <main className="l-main">
        <section className="p-strip">
          <div className="row">
            <h1>My pastes</h1>
            <MyPastesList />
          </div>
        </section>
      </main>
    </>
  )
}
