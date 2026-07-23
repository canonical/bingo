import { useState } from 'react'
import { Button, Row, Col } from '@canonical/react-components'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { tomorrow } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { PasteResponse } from '../../api/types'
import { sanitizeContent, sanitizeTitle } from '../../utils/sanitize'
import { getBasePath } from '../../utils/basePath'

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

// legacyCopy copies text via a hidden textarea + document.execCommand, for
// browsers/contexts where the async Clipboard API is unavailable (e.g. plain
// HTTP served from a non-localhost host, which is not a "secure context").
function legacyCopy(text: string): boolean {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  let success: boolean
  try {
    success = document.execCommand('copy')
  } catch {
    success = false
  }
  document.body.removeChild(textarea)
  return success
}

export default function PasteViewer({ paste, onDelete }: Props) {
  const [wrapLines, setWrapLines] = useState(false)
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied' | 'error'>('idle')
  const content = sanitizeContent(paste.content)
  const title = sanitizeTitle(paste.title)

  async function handleCopy() {
    let success = false
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(content)
        success = true
      }
    } catch {
      success = false
    }
    if (!success) {
      // Clipboard API unavailable (e.g. plain HTTP served from a non-localhost
      // host, which is not a "secure context") or it failed — fall back.
      success = legacyCopy(content)
    }
    setCopyStatus(success ? 'copied' : 'error')
    setTimeout(() => setCopyStatus('idle'), 2000)
  }

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

          {/* Action bar — nav links and interactive buttons */}
          <ul className="p-inline-list" role="list" style={{ marginBlock: '1rem' }}>
           <li className="p-inline-list__item">
             <a
               href={paste.raw_url}
               className="p-button--base is-small"
               aria-label="View raw"
             >
               View raw
             </a>
           </li>
           <li className="p-inline-list__item">
             <a
               href={`${getBasePath()}/`}
               className="p-button--base is-small"
               aria-label="New paste"
             >
               New paste
             </a>
           </li>
           <li className="p-inline-list__item">
             <Button
               type="button"
               appearance="base"
               small
               onClick={() => setWrapLines((w) => !w)}
               aria-pressed={wrapLines}
             >
               {wrapLines ? 'Unwrap lines' : 'Wrap lines'}
             </Button>
           </li>
           <li className="p-inline-list__item">
             <Button
               type="button"
               appearance="base"
               small
               aria-label="Copy to clipboard"
               aria-live="polite"
               onClick={handleCopy}
             >
               {copyStatus === 'copied' ? 'Copied!' : copyStatus === 'error' ? 'Copy failed' : 'Copy'}
             </Button>
           </li>
           {onDelete && (
             <li className="p-inline-list__item">
               <Button
                 type="button"
                 appearance="negative"
                 small
                 onClick={onDelete}
                 aria-label="Delete paste"
               >
                 Delete
               </Button>
             </li>
           )}
          </ul>

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
