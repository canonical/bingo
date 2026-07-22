import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { Spinner, Notification } from '@canonical/react-components'
import AppNavigation from '../components/Navigation/Navigation'
import PasteViewer from '../components/PasteViewer/PasteViewer'
import { getPaste, deletePaste } from '../api/client'
import { PasteResponse } from '../api/types'
import { useMe } from '../hooks/useMe'

export default function PastePage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [paste, setPaste] = useState<PasteResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { authenticated: isAuthenticated, auth_enabled: authEnabled } = useMe()

  useEffect(() => {
    if (!key) return
    getPaste(key)
      .then((result) => {
        if (result === null) {
          setNotFound(true)
        } else {
          setPaste(result)
        }
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load paste.')
      })
      .finally(() => setLoading(false))
  }, [key])

  async function handleDelete() {
    if (!key) return
    try {
      await deletePaste(key)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete paste.')
    }
  }

  return (
    <>
      <AppNavigation isAuthenticated={isAuthenticated} authEnabled={authEnabled} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-12">
              {loading && <Spinner role="status" text="Loading…" />}
              {notFound && (
                <p>
                  Paste not found or has expired.{' '}
                  <Link to="/">Create a new paste.</Link>
                </p>
              )}
              {error && (
                <Notification severity="negative" title="Error">
                  {error}
                </Notification>
              )}
              {paste && (
                <PasteViewer
                  paste={paste}
                  onDelete={isAuthenticated ? handleDelete : undefined}
                />
              )}
            </div>
          </div>
        </section>
      </main>
    </>
  )
}
