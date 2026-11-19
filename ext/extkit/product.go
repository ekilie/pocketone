package extkit

// Command is the name users type to run the binary, used wherever an extension tells them what to run next ("run
// 'pocketone login prod'") or names itself (the API token a CLI login creates). A build that ships under another
// name sets it once, before the app starts:
//
//	extkit.Command = "pockethost"
//
// or at link time: go build -ldflags "-X pocketone.dev/ext/extkit.Command=pockethost".
var Command = "pocketone"

// Version is the build's product version ("0.0.2", "dev+87f81fd2"), for remote/v1's status. The binary sets it once
// (the stock binary from pocker.Version()).
var Version = "dev"

// EnvPrefix starts every environment variable PocketOne reads, in every build: PB_ is PocketBase's own (core), P1_
// is PocketOne's, and a build on top of PocketOne uses its own prefix for its own variables (PocketHost: PH_).
// A variable's prefix names the layer that reads it, never the binary's name.
const EnvPrefix = "P1_"

// EnvVar is PocketOne's environment variable for suffix: EnvVar("TOKEN") is P1_TOKEN.
func EnvVar(suffix string) string { return EnvPrefix + suffix }
