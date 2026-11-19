// Package autotls is a PocketOne extension: automatic HTTPS certificates through ACME, with CertMagic (Caddy's
// certificate engine). Two entry points:
//
//   - On a multisite host (any runtime), [New] is a host extension that reads pocketone.yaml's `autotls` section;
//     certificate files (`server.tls.cert`/`key`) stay a core feature.
//
//     multisite.FromConfig(multisite.Options{Extensions: []multisite.Extension{autotls.New()}})
//
//   - On a single app, [NewApp] is an [extkit.Extension] that replaces PocketBase's built-in autocert on
//     `pocketbase serve example.com` (or `--https`) with CertMagic: wildcards through DNS-01, other CAs, extra domains.
//
//     extkit.MustRegister(app, autotls.NewApp(autotls.Config{Auto: true, Email: "ops@example.com"}))
package autotls

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	tlsv1 "pocketone.dev/ext/contracts/tls/v1"
	"pocketone.dev/ext/extkit"
	"strings"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
	"pocketone.dev/ext/multisite"
)

// Config is the `autotls` section (pocketone.yaml), or the settings given in Go.
type Config struct {
	Auto    bool       `yaml:"auto" json:"auto" doc:"Get certificates through ACME (CertMagic). Flag: --autotls."`
	Email   string     `yaml:"email" json:"email" doc:"ACME account email, for expiry notices from the CA. Flag: --autotlsEmail."`
	CA      string     `yaml:"ca" json:"ca" doc:"letsencrypt (default), letsencrypt-staging, zerossl, or an ACME directory URL. Flag: --autotlsCA."`
	Storage string     `yaml:"storage" json:"storage" doc:"Where certificates are kept. Default pb_data/.autotls."`
	Domains []string   `yaml:"domains" json:"domains" doc:"Extra domains to get at start; wildcards (*.example.com) need dns. Flag: --autotlsDomains."`
	DNS     *DNSConfig `yaml:"dns" json:"dns"`
}

// AutoTLS is the host extension: it provides tls/v1 when `autotls.auto` is true, and nothing otherwise.
type AutoTLS struct {
	// Config may be set before the server starts; the config file's `tls:` block is used when Auto is false here.
	Config Config

	certFiles bool            // tls.cert/tls.key are set (not allowed together with auto)
	host      *multisite.Host // its DataRoot holds the default storage, <data root>/.pocker-certs
}

// New returns the extension.
func New() *AutoTLS { return &AutoTLS{} }

// Name is the extension's name.
func (a *AutoTLS) Name() string { return "autotls" }

// RegisterHost reads pocketone.yaml's `autotls` section (unless enabled in Go) and checks it without touching the
// network. It's a host extension: with `autotls.auto` on it provides tls/v1, otherwise nothing.
func (a *AutoTLS) RegisterHost(h *multisite.Host) error {
	if err := a.configure(h); err != nil {
		return err
	}
	if a.Config.Auto {
		tlsv1.Contract.ProvideIn(h.Scope, a)
	}
	return nil
}

// Contracts implements extkit.Declarer.
func (a *AutoTLS) Contracts() extkit.Contracts {
	return extkit.Contracts{Implements: extkit.IDs("tls/v1")}
}

func (a *AutoTLS) configure(h *multisite.Host) error {
	if !a.Config.Auto {
		if c, found := Section.Get(); found {
			a.Config = c
		}
	}
	a.host = h
	a.certFiles = h.Listeners.TLS.Cert != "" || h.Listeners.TLS.Key != ""
	if !a.Config.Auto {
		return nil
	}
	return validateAutoTLS(a.Config, a.certFiles)
}

// TLS starts the certificate manager when auto is on; otherwise it returns nothing and the server falls back to
// certificate files or plain HTTP.
func (a *AutoTLS) TLS(ctx context.Context, served func(host string) bool) (*tls.Config, func(http.Handler) http.Handler, error) {
	if !a.Config.Auto {
		return nil, nil, nil
	}
	storage := ".pocker-certs"
	if a.host != nil && a.host.DataRoot != "" {
		storage = filepath.Join(a.host.DataRoot, storage)
	}
	auto, err := newAutoTLS(ctx, a.Config, a.certFiles, storage, served, true)
	if err != nil {
		return nil, nil, err
	}
	return auto.tlsConfig(), auto.httpHandler, nil
}

// AutoTLS obtains and renews certificates automatically with ACME (Let's Encrypt by default), using CertMagic,
// Caddy's certificate engine. Enabled with `tls: { auto: true }`.
//
//   - Without a DNS provider, every hostname the instance source maps gets its own certificate on its first TLS
//     handshake (HTTP-01 / TLS-ALPN-01 challenges). Works with any DNS host; no API token needed.
//
//   - With `dns:` (e.g. Cloudflare), `domains` may include wildcards (`*.example.com`), proved through DNS-01.
//
//     tls:
//     auto: true
//     email: ops@example.com
//     ca: letsencrypt            # letsencrypt (default), letsencrypt-staging, zerossl, or an ACME directory URL
//     storage: ./pb_root/.pocker-certs
//     domains: ["*.example.com", "example.com"]   # obtained up front; wildcards need dns
//     dns: { provider: cloudflare, api_token_env: CF_API_TOKEN }
//     http_listen: ":80"         # HTTP-01 challenges + redirect to HTTPS
type DNSConfig struct {
	Provider    string `yaml:"provider" json:"provider" doc:"DNS provider for DNS-01 challenges (needed for wildcards): cloudflare. Flag: --autotlsDNS."`
	APIToken    string `yaml:"api_token" json:"api_token" doc:"The provider's API token. Write $VAR to read it from the .env beside the file."`
	APITokenEnv string `yaml:"api_token_env" json:"api_token_env" doc:"Or: the environment variable holding the token. Default CF_API_TOKEN. Flag: --autotlsDNSTokenEnv."`
}

// SupportedDNSProviders are the dns.provider values the extension understands.
var SupportedDNSProviders = []string{"cloudflare"}

func (d *DNSConfig) provider() (certmagic.DNSProvider, error) {
	token := d.APIToken
	if token == "" && d.APITokenEnv != "" {
		token = os.Getenv(d.APITokenEnv)
	}
	switch strings.ToLower(d.Provider) {
	case "cloudflare":
		if token == "" {
			return nil, errors.New("tls.dns: cloudflare needs api_token or api_token_env (a token with Zone.DNS edit)")
		}
		return &cloudflare.Provider{APIToken: token}, nil
	case "":
		return nil, errors.New("tls.dns: provider is required")
	default:
		return nil, fmt.Errorf("tls.dns: unknown provider %q (supported: %s)", d.Provider, strings.Join(SupportedDNSProviders, ", "))
	}
}

func caURL(ca string) string {
	switch strings.ToLower(ca) {
	case "", "letsencrypt":
		return certmagic.LetsEncryptProductionCA
	case "letsencrypt-staging", "staging":
		return certmagic.LetsEncryptStagingCA
	case "zerossl":
		return certmagic.ZeroSSLProductionCA
	default:
		return ca
	}
}

// coveredBy reports whether name is one of domains, or matches a one-level wildcard in them.
func coveredBy(name string, domains []string) bool {
	name = normalizeHost(name)
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == name {
			return true
		}
		if strings.HasPrefix(d, "*.") {
			base := d[1:] // ".example.com"
			if strings.HasSuffix(name, base) && !strings.Contains(strings.TrimSuffix(name, base), ".") && len(name) > len(base) {
				return true
			}
		}
	}
	return false
}

type autoTLS struct {
	magic  *certmagic.Config
	issuer *certmagic.ACMEIssuer
}

// validateAutoTLS checks the auto-TLS settings without touching the network.
func validateAutoTLS(cfg Config, certFiles bool) error {
	if certFiles {
		return errors.New("tls: use either cert/key files or auto, not both")
	}
	if cfg.DNS != nil {
		if _, err := cfg.DNS.provider(); err != nil {
			return err
		}
	}
	for _, d := range cfg.Domains {
		if strings.Contains(d, "*") && cfg.DNS == nil {
			return fmt.Errorf("tls.domains: wildcard %q needs a DNS provider (tls.dns); per-hostname certificates work without one", d)
		}
	}
	return nil
}

// newAutoTLS builds the certificate manager. allowHost decides which hostnames may get an on-demand certificate
// (the instance source's mapped hosts), so random Host names can't trigger issuance. httpChallenge is false when
// something else answers port 80 (a single app: PocketBase's own redirect server), leaving TLS-ALPN-01 and DNS-01.
func newAutoTLS(ctx context.Context, cfg Config, certFiles bool, defaultStorage string, allowHost func(string) bool, httpChallenge bool) (*autoTLS, error) {
	if err := validateAutoTLS(cfg, certFiles); err != nil {
		return nil, err
	}
	storage := cfg.Storage
	if storage == "" {
		storage = defaultStorage
	}
	storage, err := filepath.Abs(storage)
	if err != nil {
		return nil, err
	}

	var magic *certmagic.Config
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) { return magic, nil },
	})
	magic = certmagic.New(cache, certmagic.Config{
		Storage: &certmagic.FileStorage{Path: storage},
		OnDemand: &certmagic.OnDemandConfig{
			DecisionFunc: func(_ context.Context, name string) error {
				if coveredBy(name, cfg.Domains) || allowHost(name) {
					return nil
				}
				return fmt.Errorf("no instance serves %q", name)
			},
		},
	})

	tmpl := certmagic.ACMEIssuer{CA: caURL(cfg.CA), Email: cfg.Email, Agreed: true, DisableHTTPChallenge: !httpChallenge}
	if cfg.DNS != nil {
		p, err := cfg.DNS.provider()
		if err != nil {
			return nil, err
		}
		tmpl.DNS01Solver = &certmagic.DNS01Solver{DNSManager: certmagic.DNSManager{DNSProvider: p}}
	}
	issuer := certmagic.NewACMEIssuer(magic, tmpl)
	magic.Issuers = []certmagic.Issuer{issuer}

	if len(cfg.Domains) > 0 {
		if err := magic.ManageAsync(ctx, cfg.Domains); err != nil {
			return nil, fmt.Errorf("tls.domains: %w", err)
		}
	}
	return &autoTLS{magic: magic, issuer: issuer}, nil
}

func (a *autoTLS) tlsConfig() *tls.Config {
	c := a.magic.TLSConfig()
	c.MinVersion = tls.VersionTLS12
	c.NextProtos = append([]string{"h2", "http/1.1"}, c.NextProtos...) // keep acme-tls/1 for TLS-ALPN challenges
	return c
}

// httpHandler answers ACME HTTP-01 challenges, passing everything else to next.
func (a *autoTLS) httpHandler(next http.Handler) http.Handler {
	return a.issuer.HTTPChallengeHandler(next)
}

func normalizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.TrimSuffix(h, ".")
}

// Section is the key of the `autotls` section (pocketone.yaml): automatic certificates for a multisite host.
var Section = extkit.NewSection("autotls", Config{}, "Automatic HTTPS certificates (ACME): auto, email, CA, domains, DNS-01, certificate storage. The --autotls* flags win.")
