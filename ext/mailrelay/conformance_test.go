package mailrelay_test

import (
	"context"
	"testing"

	"pocketone.dev/ext/extkit"
	"pocketone.dev/ext/extkit/conformance"
	"pocketone.dev/ext/mailrelay"
)

type nopUpstream struct{}

func (nopUpstream) Send(context.Context, string, []string, []byte) error { return nil }

func TestConformance(t *testing.T) {
	conformance.Run(t, func() extkit.Extension {
		return mailrelay.New(mailrelay.Config{Domain: "mail.test", Upstream: nopUpstream{}})
	})
}
