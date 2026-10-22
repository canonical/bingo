import AppNavigation from '../components/Navigation/Navigation'
import { getBasePath } from '../utils/basePath'

export default function SignInPage() {
  return (
    <>
      <AppNavigation isAuthenticated={false} authEnabled={false} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-6">
              <h1 className="p-heading--2">Sign in to Bingo</h1>
              <p>You need to sign in to create and view pastes.</p>
              <a className="p-button--positive" href={`${getBasePath()}/auth/login`}>
                Sign in with OIDC
              </a>
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
