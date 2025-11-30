package lib_test

import (
	"testing"

	"github.com/matryer/is"
)

func TestPostMe_NoBody(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "gram@example.com")
	r := u.DoUser("POST", "/users/me", "")
	EqBody(is, r, `{"errors": [{
		"detail": "cannot parse request: EOF"
	}]}`)
	is.Equal(r.StatusCode, 400)
}

func TestPostMe_EmptyObject(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "gram@example.com")
	r := u.DoUser("POST", "/users/me", "{}")
	EqBody(is, r, `{"errors": [{
		"detail": "cannot parse request: data field not found in request body"
	}]}`)
	is.Equal(r.StatusCode, 400)
}

func TestPostMe(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "grammar@example.com")
	r := u.DoUser("POST", "/users/me", `{"data": {
		"type": "me",
		"attributes": {
			"name": "grammar",
			"language": "en",
			"country": "NL",
			"timezone": "Europe/Amsterdam"
		}
	}}`)
	EqBody(is, r, `{"data": {
		"type": "me",
		"id": ANY,
		"attributes": {
			"email": "grammar@example.com",
			"name": "grammar",
			"language": "en",
			"country": "NL",
			"timezone": "Europe/Amsterdam",
			"publisher": false,
			"created_at": ANY,
			"updated_at": ANY
		}
	}}`)
	is.Equal(r.StatusCode, 201)
}

func TestGetMe_Unregistered(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "grammar@example.com")
	r := u.GetUser("/users/me")
	EqBody(is, r, `{"errors": [{
		"detail": "user registration is not complete",
		"code": "unregistered"
	}]}`)
	is.Equal(r.StatusCode, 401)
}

func TestGetMe(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "grammar@example.com")
	u.Register()
	r := u.GetUser("/users/me")
	EqBody(is, r, `{"data": {
		"type": "me",
		"id": ANY,
		"attributes": ANY
	}}`)
	is.Equal(r.StatusCode, 200)
}

func TestDeleteMe(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	c := NewTestClient(t)
	u := NewTestUser(c, "gram@example.com")
	u.Register()
	r := u.DoUser("DELETE", "/users/me", ``)
	is.Equal(r.StatusCode, 204)
}
