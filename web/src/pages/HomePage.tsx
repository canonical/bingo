import { useNavigate } from 'react-router-dom'
import Navigation from '../components/Navigation/Navigation'
import NewPasteForm from '../components/NewPasteForm/NewPasteForm'

export default function HomePage() {
  const navigate = useNavigate()
  const isAuthenticated = document.cookie.includes('csrf_token=')

  return (
    <>
      <Navigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip">
          <div className="row">
            <NewPasteForm onCreated={(key) => navigate(`/${key}`)} />
          </div>
        </section>
      </main>
    </>
  )
}
