import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Notification, Spinner } from '@canonical/react-components'
import { getMyPastes } from '../../api/client'
import { PasteListItem } from '../../api/types'
import { sanitizeTitle } from '../../utils/sanitize'

export default function MyPastesList() {
  const [pastes, setPastes] = useState<PasteListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getMyPastes()
      .then((resp) => setPastes(resp.pastes))
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load pastes.'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spinner role="status" text="Loading your pastes…" />
  if (error) return <Notification severity="negative" title="Error" role="alert">{error}</Notification>
  if (pastes.length === 0) return <p>No pastes yet. <Link to="/">Create one!</Link></p>

  return (
    <ul className="p-list">
      {pastes.map((p) => (
        <li key={p.key} className="p-list__item">
          <Link to={`/${p.key}`} aria-label={p.key}>
            {sanitizeTitle(p.title) || p.key}
          </Link>
          {' — '}
          <span>{p.language}</span>
          {' — '}
          <time dateTime={p.created_at}>{new Date(p.created_at).toLocaleDateString()}</time>
        </li>
      ))}
    </ul>
  )
}
