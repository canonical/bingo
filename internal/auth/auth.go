// Package auth provides optional OIDC authentication middleware.
// Implementation is added in Phase 3.
package auth

import (
	_ "github.com/coreos/go-oidc/v3/oidc"
	_ "golang.org/x/oauth2"
	_ "github.com/gorilla/securecookie"
)
