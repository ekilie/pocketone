package clicmdv1

import "testing"

func TestSourceOf(t *testing.T) {
	for name, want := range map[string]string{
		"github.com/pockethost/pocketone/ext/pitr.(*PITR).Command.func1":                   "ext/pitr",
		"github.com/pockethost/pocketone/ext/tokens/oauth.New.func2":                       "ext/tokens",
		"github.com/pockethost/pocketone/ext/contracts/remote/v1/remoteclient.Login.func1": "contracts/remote/v1",
		"github.com/pockethost/pocketone/ext/contracts/mail/suppression/v1.x.func1":        "contracts/mail.suppression/v1",
		"ganigeorgiev.com/pocketbase/pocketbase/cmd.NewServeCommand.func1":                 "pocketbase",
		"main.main.func1": "main",
	} {
		if got := sourceOfPackage(packageOf(name)); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
	if got := sourceOf(TestSourceOf); got != "contracts/cli.commands/v1" {
		t.Errorf("sourceOf(a func here) = %q", got)
	}
}
