import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Spinner, Notification } from '@canonical/react-components'
import AppNavigation from '../components/Navigation/Navigation'
import PasteViewer from '../components/PasteViewer/PasteViewer'
import { getPaste, deletePaste } from '../api/client'
import { PasteResponse, ApiRequestError } from '../api/types'

export default function PastePage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [paste, setPaste] = useState<PasteResponse | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const isAuthenticated = document.cookie.includes('csrf_token=')

  useEffect(() => {
    if (!key) return
    getPaste(key)
      .then(setPaste)
      .catch((err) => {
        if (err instanceof ApiRequestError && err.status === 404) {
          setNotFound(true)
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load paste.')
        }
      })
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
      <AppNavigation isAuthenticated={isAuthenticated} />
      <main className="l-main">
        <section className="p-strip is-shallow">
          <div className="row">
            <div className="col-12">
              {!paste && !notFound && !error && <Spinner role="status" text="Loading…" />}
              {notFound && (
                <p>
                  Paste not found or has expired.{' '}
                  <a href="/">Create a new paste.</a>
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
