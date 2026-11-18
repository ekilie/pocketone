// Package s3test is an S3-compatible server for tests: in memory, in the test process, nothing to install, so tests
// that need a bucket run everywhere (CI included) instead of skipping.
//
// It serves PocketBase's own S3 client as a bucket would (requests carrying an Authorization header are accepted as
// they are), and it checks every presigned download the way a real bucket does, from the request it received: the
// access key, the expiry, and the SigV4 signature over the method, path, query and host it was actually sent. A
// download with neither an Authorization header nor a valid signature gets 403, so a redirect that isn't signed for
// exactly that object fails the test.
//
//	srv := s3test.Start(t, "shared", "own")
//	// endpoint srv.URL, region srv.Region, keys srv.AccessKey / srv.Secret, path-style addressing
package s3test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// Server is a running in-memory bucket server. Use path-style addressing (force_path_style / forcePathStyle).
type Server struct {
	URL       string // http://127.0.0.1:<port>
	Region    string
	AccessKey string
	Secret    string

	presigned atomic.Int64
}

// Start runs a server with the given buckets until the test ends.
func Start(t testing.TB, buckets ...string) *Server {
	t.Helper()
	be := s3mem.New()
	for _, b := range buckets {
		if err := be.CreateBucket(b); err != nil {
			t.Fatalf("s3test: bucket %q: %v", b, err)
		}
	}
	s := &Server{Region: "us-east-1", AccessKey: "s3test", Secret: "s3test-secret"}
	fake := gofakes3.New(be).Server()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("X-Amz-Signature") != "":
			if err := s.verify(r); err != nil {
				http.Error(w, "s3test: "+err.Error(), http.StatusForbidden)
				return
			}
			s.presigned.Add(1)
			// gofakes3 ignores the signing parameters; the writer applies the response-* overrides, as S3 does
			fake.ServeHTTP(&overrideWriter{ResponseWriter: w, q: q}, r)
		case r.Header.Get("Authorization") != "":
			fake.ServeHTTP(w, r)
		default:
			http.Error(w, "s3test: anonymous request", http.StatusForbidden)
		}
	}))
	t.Cleanup(hs.Close)
	s.URL = hs.URL
	return s
}

// Presigned is how many presigned requests the server has served so far (each one verified).
func (s *Server) Presigned() int { return int(s.presigned.Load()) }

// overrideWriter sets S3's response-* overrides (response-content-type, response-content-disposition, ...) on the
// response, as S3 does for a signed request.
type overrideWriter struct {
	http.ResponseWriter
	q    url.Values
	done bool
}

func (o *overrideWriter) apply() {
	if o.done {
		return
	}
	o.done = true
	names := map[string]string{
		"response-content-type": "Content-Type", "response-content-disposition": "Content-Disposition",
		"response-cache-control": "Cache-Control", "response-content-language": "Content-Language",
		"response-content-encoding": "Content-Encoding", "response-expires": "Expires",
	}
	for k, h := range names {
		if v := o.q.Get(k); v != "" {
			o.Header().Set(h, v)
		}
	}
}

func (o *overrideWriter) WriteHeader(code int) {
	if code < 300 {
		o.apply()
	}
	o.ResponseWriter.WriteHeader(code)
}

func (o *overrideWriter) Write(b []byte) (int, error) {
	if !o.done {
		o.WriteHeader(http.StatusOK)
	}
	return o.ResponseWriter.Write(b)
}

// verify checks a query-signed request the way S3 does: the algorithm, the credential's key and scope, the expiry,
// and the signature over the canonical request built from what was received.
func (s *Server) verify(r *http.Request) error {
	q := r.URL.Query()
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		return fmt.Errorf("algorithm %q", q.Get("X-Amz-Algorithm"))
	}
	cred := strings.Split(q.Get("X-Amz-Credential"), "/")
	if len(cred) != 5 || cred[0] != s.AccessKey || cred[2] != s.Region || cred[3] != "s3" || cred[4] != "aws4_request" {
		return fmt.Errorf("credential %q", q.Get("X-Amz-Credential"))
	}
	at, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	if err != nil || at.Format("20060102") != cred[1] {
		return fmt.Errorf("date %q", q.Get("X-Amz-Date"))
	}
	expires, err := strconv.Atoi(q.Get("X-Amz-Expires"))
	if err != nil || expires < 1 || expires > 7*24*3600 {
		return fmt.Errorf("expires %q", q.Get("X-Amz-Expires"))
	}
	if time.Now().After(at.Add(time.Duration(expires) * time.Second)) {
		return fmt.Errorf("expired")
	}
	if at.After(time.Now().Add(15 * time.Minute)) {
		return fmt.Errorf("signed in the future")
	}

	signed := strings.Split(q.Get("X-Amz-SignedHeaders"), ";")
	sort.Strings(signed)
	var canonHeaders strings.Builder
	for _, h := range signed {
		v := r.Header.Get(h)
		if h == "host" {
			v = r.Host
		}
		canonHeaders.WriteString(h + ":" + strings.TrimSpace(v) + "\n")
	}

	var pairs []string
	for k, vs := range q {
		if k == "X-Amz-Signature" {
			continue
		}
		for _, v := range vs {
			pairs = append(pairs, encode(k, true)+"="+encode(v, true))
		}
	}
	sort.Strings(pairs)

	path := r.URL.EscapedPath()
	canonical := strings.Join([]string{r.Method, path, strings.Join(pairs, "&"), canonHeaders.String(),
		strings.Join(signed, ";"), "UNSIGNED-PAYLOAD"}, "\n")
	scope := strings.Join(cred[1:], "/")
	sum := sha256.Sum256([]byte(canonical))
	toSign := strings.Join([]string{"AWS4-HMAC-SHA256", q.Get("X-Amz-Date"), scope, hex.EncodeToString(sum[:])}, "\n")
	key := []byte("AWS4" + s.Secret)
	for _, part := range []string{cred[1], cred[2], "s3", "aws4_request"} {
		key = mac(key, part)
	}
	if want := hex.EncodeToString(mac(key, toSign)); !hmac.Equal([]byte(want), []byte(q.Get("X-Amz-Signature"))) {
		return fmt.Errorf("signature does not match (canonical request:\n%s)", canonical)
	}
	return nil
}

func mac(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}

// encode is SigV4's URI encoding: every byte but A-Z a-z 0-9 - _ . ~ is percent-encoded (and "/" too when
// encodeSlash).
func encode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
