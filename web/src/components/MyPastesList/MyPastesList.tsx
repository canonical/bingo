import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { MainTable, Notification, Spinner } from '@canonical/react-components'
import { getMyPastes } from '../../api/client'
import { PasteListItem } from '../../api/types'
import { sanitizeTitle } from '../../utils/sanitize'

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString('en-GB', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  } catch {
    return iso
  }
}

export default function MyPastesList() {
  const [pastes, setPastes] = useState<PasteListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getMyPastes()
      .then((resp) => setPastes(resp.pastes))
      .catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to load pastes.'),
      )
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spinner role="status" text="Loading your pastes…" />
  if (error)
    return (
      <Notification severity="negative" title="Error" role="alert">
        {error}
      </Notification>
    )

  const headers = [
    { content: 'Title', sortKey: 'title' },
    { content: 'Language', sortKey: 'language' },
    { content: 'Size', sortKey: 'size_bytes' },
    { content: 'Created', sortKey: 'created_at' },
    { content: 'Expires', sortKey: 'expires_at' },
  ]

  const rows = pastes.map((p) => ({
    key: p.key,
    sortData: {
      title: sanitizeTitle(p.title) || p.key,
      language: p.language,
      size_bytes: p.size_bytes,
      created_at: p.created_at,
      expires_at: p.expires_at,
    },
    columns: [
      {
        content: (
          <Link to={`/${p.key}`}>
            {sanitizeTitle(p.title) || p.key}
          </Link>
        ),
      },
      { content: p.language },
      { content: `${p.size_bytes} B` },
      { content: formatDate(p.created_at) },
      { content: formatDate(p.expires_at) },
    ],
  }))

  return (
    <MainTable
      headers={headers}
      rows={rows}
      sortable
      emptyStateMsg="No pastes yet."
    />
  )
}
