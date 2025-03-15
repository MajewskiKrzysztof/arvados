// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package localdb

import (
	"database/sql"

	"git.arvados.org/arvados.git/lib/ctrlctx"
	"git.arvados.org/arvados.git/sdk/go/arvados"
	"git.arvados.org/arvados.git/sdk/go/arvadostest"
	check "gopkg.in/check.v1"
)

var _ = check.Suite(&TestUserSuite{})

type TestUserSuite struct {
	localdbSuite
}

func (s *TestUserSuite) SetUpTest(c *check.C) {
	s.localdbSuite.SetUpTest(c)
	s.cluster.Login.Test.Enable = true
	s.cluster.Login.Test.Users = map[string]arvados.TestUser{
		"valid": {Email: "valid@example.com", Password: "v@l1d"},
	}
	s.localdb.loginController = &testLoginController{
		Cluster: s.cluster,
		Parent:  s.localdb,
	}
}

func (s *TestUserSuite) TestLogin(c *check.C) {
	for _, trial := range []struct {
		success  bool
		username string
		password string
	}{
		{false, "foo", "bar"},
		{false, "", ""},
		{false, "valid", ""},
		{false, "", "v@l1d"},
		{true, "valid", "v@l1d"},
		{true, "valid@example.com", "v@l1d"},
	} {
		c.Logf("=== %#v", trial)
		resp, err := s.localdb.UserAuthenticate(s.ctx, arvados.UserAuthenticateOptions{
			Username: trial.username,
			Password: trial.password,
		})
		if trial.success {
			c.Check(err, check.IsNil)
			c.Check(resp.APIToken, check.Not(check.Equals), "")
			c.Check(resp.UUID, check.Matches, `zzzzz-gj3su-.*`)
			c.Check(resp.Scopes, check.DeepEquals, []string{"all"})

			authinfo := getCallbackAuthInfo(c, s.railsSpy)
			c.Check(authinfo.Email, check.Equals, "valid@example.com")
			c.Check(authinfo.AlternateEmails, check.DeepEquals, []string(nil))
		} else {
			c.Check(err, check.ErrorMatches, `authentication failed.*`)
		}
	}
}

func (s *TestUserSuite) TestLoginForm(c *check.C) {
	resp, err := s.localdb.Login(s.ctx, arvados.LoginOptions{
		ReturnTo: "https://localhost:12345/example",
	})
	c.Check(err, check.IsNil)
	c.Check(resp.HTML.String(), check.Matches, `(?ms).*<form method="POST".*`)
	c.Check(resp.HTML.String(), check.Matches, `(?ms).*<input id="return_to" type="hidden" name="return_to" value="https://localhost:12345/example">.*`)
}

func (s *TestUserSuite) TestExpireTokenOnLogout(c *check.C) {
	s.cluster.Login.TrustPrivateNetworks = true
	returnTo := "https://[::1]:12345/logout"
	for _, trial := range []struct {
		requestToken      string
		expiringTokenUUID string
		shouldExpireToken bool
	}{
		// v2 token
		{arvadostest.ActiveTokenV2, arvadostest.ActiveTokenUUID, true},
		// v1 token
		{arvadostest.AdminToken, arvadostest.AdminTokenUUID, true},
		// inexistent v1 token -- logout shouldn't fail
		{"thisdoesntexistasatoken", "", false},
		// inexistent v2 token -- logout shouldn't fail
		{"v2/some-fake-uuid/thisdoesntexistasatoken", "", false},
	} {
		c.Logf("=== %#v", trial)
		ctx := ctrlctx.NewWithToken(s.ctx, s.cluster, trial.requestToken)

		var tokenUUID string
		var err error
		qry := `SELECT uuid FROM api_client_authorizations
			WHERE uuid=$1
				AND (least(expires_at, refreshes_at) IS NULL OR least(expires_at, refreshes_at) > current_timestamp AT TIME ZONE 'UTC')
			LIMIT 1`

		if trial.shouldExpireToken {
			err = s.tx.QueryRowContext(ctx, qry, trial.expiringTokenUUID).Scan(&tokenUUID)
			c.Check(err, check.IsNil)
		}

		resp, err := s.localdb.Logout(ctx, arvados.LogoutOptions{
			ReturnTo: returnTo,
		})
		c.Check(err, check.IsNil)
		c.Check(resp.RedirectLocation, check.Equals, returnTo)

		if trial.shouldExpireToken {
			err = s.tx.QueryRowContext(ctx, qry, trial.expiringTokenUUID).Scan(&tokenUUID)
			c.Check(err, check.Equals, sql.ErrNoRows)
		}
	}
}
