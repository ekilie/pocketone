package mailrelay_test

import (
	"context"
	"testing"

	"github.com/pockethost/pocketone/ext/core/multisite-mailrelay"
	"github.com/pockethost/pocketone/ext/core/pluginkit"
	"github.com/pockethost/pocketone/ext/core/pluginkit/conformance"
)

type nopUpstream struct{}

func (nopUpstream) Send(context.Context, string, []string, []byte) error { return nil }

func TestConformance(t *testing.T) {
	conformance.Run(t, func() pluginkit.Plugin {
		return mailrelay.New(mailrelay.Config{Domain: "mail.test", Upstream: nopUpstream{}})
	})
}
