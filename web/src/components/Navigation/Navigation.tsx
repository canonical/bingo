import { Navigation, Theme } from '@canonical/react-components'
import { Link } from 'react-router-dom'
import type { NavItem, GenerateLink } from '@canonical/react-components'

interface Props {
  isAuthenticated: boolean
  userEmail?: string
  // Whether OIDC auth is configured on the server. Defaults to true so
  // existing callers that don't pass it keep showing "Log in" as before.
  authEnabled?: boolean
}

// generateLink is called for items with a `url`. Use react-router Link for
// internal paths; fall through to a plain <a> for /auth/* server routes.
const generateLink: GenerateLink = ({ label, url, ...props }) => {
  if (url && !url.startsWith('/auth')) {
    return <Link to={url} {...(props as object)}>{label}</Link>
  }
  return <a href={url} {...(props as object)}>{label}</a>
}

export default function AppNavigation({ isAuthenticated, userEmail, authEnabled = true }: Props) {
  const rightItems: NavItem[] = isAuthenticated
    ? [
        { label: 'My pastes', url: '/my-pastes' },
        ...(userEmail ? [{ label: userEmail, url: '#' } as NavItem] : []),
        { label: 'Log out', url: '/auth/logout' },
      ]
    : authEnabled
      ? [{ label: 'Log in', url: '/auth/login' }]
      : []

  return (
    <Navigation
      theme={Theme.DARK}
      logo={{
        src: 'https://assets.ubuntu.com/v1/82818827-CoF_white.svg',
        title: 'Bingo',
        url: '/',
      }}
      generateLink={generateLink}
      items={[{ label: 'New paste', url: '/' }]}
      itemsRight={rightItems}
    />
  )
}
