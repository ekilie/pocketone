package mailrelay_test

import (
	"context"
	"testing"

	"github.com/pockethost/pocketone/ext/extkit"
	"github.com/pockethost/pocketone/ext/extkit/conformance"
	"github.com/pockethost/pocketone/ext/multisite-mailrelay"
)

type nopUpstream struct{}

func (nopUpstream) Send(context.Context, string, []string, []byte) error { return nil }

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension {
		return mailrelay.New(mailrelay.Config{Domain: "mail.test", Upstream: nopUpstream{}})
	})
}
