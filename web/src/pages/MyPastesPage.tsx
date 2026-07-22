import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import AppNavigation from '../components/Navigation/Navigation'
import MyPastesList from '../components/MyPastesList/MyPastesList'
import { useMe } from '../hooks/useMe'

export default function MyPastesPage() {
  const navigate = useNavigate()
  const { authenticated: isAuthenticated, email } = useMe()

  useEffect(() => {
    if (!isAuthenticated) navigate('/')
  }, [isAuthenticated, navigate])

  if (!isAuthenticated) return null

  return (
    <>
      <AppNavigation isAuthenticated userEmail={email} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-12">
              <h1 className="p-heading--2">My pastes</h1>
              <MyPastesList />
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
