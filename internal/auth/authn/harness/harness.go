// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package harness

import (
	"net/http"

	"github.com/harness/gitness/internal/auth/authn"
	"github.com/harness/gitness/types"
)

var _ authn.Authenticator = (*Authenticator)(nil)

type Authenticator struct {
	// some config to validate jwt
}

func NewAuthenticator() (authn.Authenticator, error) {
	return &Authenticator{}, nil
}

func (this *Authenticator) Authenticate(r *http.Request) (*types.User, error) {
	return &types.User{}, nil
}