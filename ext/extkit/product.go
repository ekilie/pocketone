package extkit

// Command is the name users type to run the binary, used wherever an extension tells them what to run next ("run
// 'pocketone login prod'") or names itself (the API token a CLI login creates). A build that ships under another
// name sets it once, before the app starts:
//
//	extkit.Command = "pockethost"
//
// or at link time: go build -ldflags "-X github.com/pockethost/pocketone/ext/extkit.Command=pockethost".
var Command = "pocketone"

// Version is the build's product version ("0.0.2", "dev+87f81fd2"), for remote/v1's status. The binary sets it once
// (the stock binary from pocker.Version()).
var Version = "dev"
