import { useNavigate } from 'react-router-dom'
import AppNavigation from '../components/Navigation/Navigation'
import NewPasteForm from '../components/NewPasteForm/NewPasteForm'
import { useAuthEnabled } from '../hooks/useAuthEnabled'

export default function HomePage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')
  const authEnabled = useAuthEnabled()

  return (
    <>
      <AppNavigation isAuthenticated={isAuthenticated} authEnabled={authEnabled} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-8">
              <h1 className="p-heading--2">New paste</h1>
              <NewPasteForm onCreated={(key) => navigate(`/${key}`)} />
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
