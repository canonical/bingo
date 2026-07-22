/**
 * ErrorPage is shown when the app cannot reach the server at all (e.g. a
 * network failure while checking auth status in AuthGuard), rather than
 * rendering a page that would otherwise fail in confusing ways.
 */
export default function ErrorPage() {
  return (
    <main className="l-main">
      <section className="p-strip is-shallow">
        <div className="row">
          <div className="col-8">
            <h1 className="p-heading--2">Connection problem</h1>
            <p>We&apos;re having trouble connecting. Please check your connection and try again.</p>
            <button className="p-button--positive" onClick={() => window.location.reload()}>
              Reload
            </button>
          </div>
        </div>
      </section>
    </main>
  )
}
