import { Link } from 'react-router-dom'

interface Props {
  isAuthenticated: boolean
  userEmail?: string
}

export default function Navigation({ isAuthenticated, userEmail }: Props) {
  return (
    <nav className="p-navigation" aria-label="Main navigation">
      <div className="p-navigation__row">
        <div className="p-navigation__banner">
          <Link to="/" className="p-navigation__link">bingo</Link>
        </div>
        <ul className="p-navigation__items">
          <li className="p-navigation__item">
            <Link to="/" className="p-navigation__link" aria-label="New paste">New paste</Link>
          </li>
          {isAuthenticated ? (
            <>
              <li className="p-navigation__item">
                <Link to="/my-pastes" className="p-navigation__link" aria-label="My pastes">My pastes</Link>
              </li>
              {userEmail && (
                <li className="p-navigation__item">
                  <span className="p-navigation__link">{userEmail}</span>
                </li>
              )}
              <li className="p-navigation__item">
                <a href="/auth/logout" className="p-navigation__link" aria-label="Log out">Log out</a>
              </li>
            </>
          ) : (
            <li className="p-navigation__item">
              <a href="/auth/login" className="p-navigation__link" aria-label="Log in">Log in</a>
            </li>
          )}
        </ul>
      </div>
    </nav>
  )
}
