import React, { useEffect, useState } from 'react'
import { Button, Input, Select, Textarea, Notification, Spinner } from '@canonical/react-components'
import { createPaste, getLanguages } from '../../api/client'
import { CreatePasteParams } from '../../api/types'

interface Props {
  onCreated: (key: string) => void
}

const EXPIRY_OPTIONS = [
  { value: '1d', label: '1 day' },
  { value: '1w', label: '1 week' },
  { value: '1mo', label: '1 month' },
  { value: '3mo', label: '3 months' },
  { value: '1y', label: '1 year' },
] as const

export default function NewPasteForm({ onCreated }: Props) {
  const [languages, setLanguages] = useState<string[]>([])
  const [content, setContent] = useState('')
  const [title, setTitle] = useState('')
  const [language, setLanguage] = useState('text')
  const [expiresIn, setExpiresIn] = useState<CreatePasteParams['expires_in']>('3mo')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getLanguages()
      .then((langs) => {
        setLanguages(langs)
        if (langs.length > 0 && !langs.includes('text')) setLanguage(langs[0])
      })
      .catch(() => setLanguages(['text']))
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!content.trim()) {
      setError('Content is required.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const params: CreatePasteParams = { content, language, expires_in: expiresIn }
      if (title.trim()) params.title = title.trim()
      const resp = await createPaste(params)
      onCreated(resp.key)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create paste.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} aria-label="New paste form" className="p-form p-form--stacked">
      {error && (
        <Notification severity="negative" title="Error" role="alert">
          {error}
        </Notification>
      )}
      <Input
        id="title"
        label="Title (optional)"
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        maxLength={255}
      />
      <Select
        id="language"
        label="Syntax"
        value={language}
        onChange={(e) => setLanguage(e.target.value)}
        options={languages.map((l) => ({ value: l, label: l }))}
      />
      <Select
        id="expires_in"
        label="Expires in"
        value={expiresIn}
        onChange={(e) => setExpiresIn(e.target.value as CreatePasteParams['expires_in'])}
        options={EXPIRY_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
      />
      <Textarea
        id="content"
        label="Content"
        value={content}
        onChange={(e) => setContent(e.target.value)}
        rows={20}
        required
        aria-required="true"
      />
      <Button type="submit" appearance="positive" disabled={submitting}>
        {submitting ? <Spinner text="Creating…" /> : 'Create paste'}
      </Button>
    </form>
  )
}
