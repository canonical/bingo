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
          {title && <h2 className="p-heading--3">{title}</h2>}

          {/* Metadata table */}
          <table className="p-table--mobile-card u-no-margin--bottom">
            <tbody>
              <tr>
                <td className="u-text--muted">Language</td>
                <td>{paste.language}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Created</td>
                <td>{formatDate(paste.created_at)}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Expires</td>
                <td>{formatDate(paste.expires_at)}</td>
              </tr>
              <tr>
                <td className="u-text--muted">Size</td>
                <td>{paste.size_bytes} bytes</td>
              </tr>
            </tbody>
          </table>

          {/* Action bar — use <a> with Vanilla classes for navigation links,
              Pragma Button for interactive actions (copy, wrap, delete) */}
          <div className="u-sv2" style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', marginBlock: '1rem' }}>
            <a
              href={paste.raw_url}
              className="p-button--base is-small"
              aria-label="View raw"
            >
              View raw
            </a>
            <a
              href="/"
              className="p-button--base is-small"
              aria-label="New paste"
            >
              New paste
            </a>
            <Button
              type="button"
              appearance="base"
              small
              onClick={() => setWrapLines((w) => !w)}
              aria-pressed={wrapLines}
            >
              {wrapLines ? 'Unwrap lines' : 'Wrap lines'}
            </Button>
            <Button
              type="button"
              appearance="base"
              small
              aria-label="Copy to clipboard"
              onClick={() => navigator.clipboard.writeText(content)}
            >
              Copy
            </Button>
            {onDelete && (
              <Button
                type="button"
                appearance="negative"
                small
                onClick={onDelete}
                aria-label="Delete paste"
              >
                Delete
              </Button>
            )}
          </div>

          {/* Code block — SyntaxHighlighter renders tokens as React elements,
              never passes untreated API strings to dangerouslySetInnerHTML */}
          <div className="paste-code-block">
            <SyntaxHighlighter
              language={paste.language}
              style={tomorrow}
              wrapLines={wrapLines}
              wrapLongLines={wrapLines}
              showLineNumbers
            >
              {content}
            </SyntaxHighlighter>
          </div>
        </Col>
      </Row>
    </article>
  )
}
