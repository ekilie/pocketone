package llm

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	llmc "pocketone.dev/ext/contracts/llm"
)

// GuestCookie is the cookie that tells one anonymous visitor from another, so each guest gets a bucket of their own.
// Its value is a random id and a signature (HMAC-SHA256, keyed from the app's superuser token secret), so a visitor
// can't pick someone else's id.
const GuestCookie = "pocketone_llm_guest"

// guestCookieAge is how long a guest keeps their id: a year.
const guestCookieAge = 365 * 24 * 60 * 60

// SubjectOf is whom a request's calls are for: its auth record, else the guest its cookie names, else its IP. A
// request without a valid guest cookie gets one set on its response and counts by IP until the browser sends it
// back, so a client that drops cookies can't get a fresh bucket on every call.
func SubjectOf(app core.App, e *core.RequestEvent) string {
	if e == nil {
		return ""
	}
	if e.Auth != nil {
		return llmc.UserSubject(e.Auth.Collection().Name, e.Auth.Id)
	}
	key := guestKey(app)
	if c, err := e.Request.Cookie(GuestCookie); err == nil {
		if id, ok := verifyGuest(key, c.Value); ok {
			return "guest:" + id
		}
	}
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	http.SetCookie(e.Response, &http.Cookie{
		Name: GuestCookie, Value: id + "." + signGuest(key, id), Path: "/", MaxAge: guestCookieAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: e.Request.TLS != nil,
	})
	return "guest:ip:" + e.RealIP()
}

// guestKey is the signing key: derived from the superusers collection's token secret, which every app has and keeps
// private. Changing that secret signs everyone out and gives every guest a new id.
func guestKey(app core.App) []byte {
	secret := ""
	if c, err := app.FindCachedCollectionByNameOrId(core.CollectionNameSuperusers); err == nil {
		secret = c.AuthToken.Secret
	}
	h := sha256.Sum256([]byte("pocketone llm guest\x00" + secret))
	return h[:]
}

func signGuest(key []byte, id string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(id))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

func verifyGuest(key []byte, value string) (string, bool) {
	id, sig, ok := strings.Cut(value, ".")
	if !ok || len(id) != 24 {
		return "", false
	}
	return id, hmac.Equal([]byte(sig), []byte(signGuest(key, id)))
}
