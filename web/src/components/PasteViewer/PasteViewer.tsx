import { useState } from 'react'
import { Button, Row, Col } from '@canonical/react-components'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { tomorrow } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { PasteResponse } from '../../api/types'
import { sanitizeContent, sanitizeTitle } from '../../utils/sanitize'

interface Props {
  paste: PasteResponse
  onDelete?: () => void
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString('en-US', { timeZone: 'UTC' })
  } catch {
    return iso
  }
}

export default function PasteViewer({ paste, onDelete }: Props) {
  const [wrapLines, setWrapLines] = useState(false)
  const content = sanitizeContent(paste.content)
  const title = sanitizeTitle(paste.title)

  return (
    <article aria-label="Paste viewer">
      <Row>
        <Col size={12}>
          <header>
            {title && <h1>{title}</h1>}
            <dl>
              <dt>Language</dt>
              <dd>{paste.language}</dd>
              <dt>Created</dt>
              <dd>{formatDate(paste.created_at)}</dd>
              <dt>Expires</dt>
              <dd>{formatDate(paste.expires_at)}</dd>
              <dt>Size</dt>
              <dd>{paste.size_bytes} bytes</dd>
            </dl>
          </header>
          <div className="paste-actions">
            <a href={paste.raw_url} aria-label="View raw">View raw</a>
            {' · '}
            <a href="/" aria-label="New paste">New paste</a>
            {' · '}
            <button
              type="button"
              onClick={() => setWrapLines((w) => !w)}
              aria-pressed={wrapLines}
            >
              {wrapLines ? 'Unwrap' : 'Wrap'} lines
            </button>
            {' · '}
            <button
              type="button"
              aria-label="Copy to clipboard"
              onClick={() => navigator.clipboard.writeText(content)}
            >
              Copy
            </button>
            {onDelete && (
              <>
                {' · '}
                <Button
                  type="button"
                  appearance="negative"
                  small
                  onClick={onDelete}
                  aria-label="Delete paste"
                >
                  Delete
                </Button>
              </>
            )}
          </div>
          {/* §8: content passed as children (string) — SyntaxHighlighter does NOT use
              dangerouslySetInnerHTML for its own injected content; it renders tokens as
              React elements. We never pass untreated API strings to dangerouslySetInnerHTML. */}
          <SyntaxHighlighter
            language={paste.language}
            style={tomorrow}
            wrapLines={wrapLines}
            wrapLongLines={wrapLines}
            showLineNumbers
          >
            {content}
          </SyntaxHighlighter>
        </Col>
      </Row>
    </article>
  )
}
