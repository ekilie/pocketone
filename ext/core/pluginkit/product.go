package pluginkit

// Command is the name users type to run the binary, used wherever a plugin tells them what to run next ("run
// 'pocketone login prod'") or names itself (the API token a CLI login creates). A build that ships under another
// name sets it once, before the app starts:
//
//	pluginkit.Command = "pockethost"
//
// or at link time: go build -ldflags "-X github.com/pockethost/pocketone/ext/core/pluginkit.Command=pockethost".
var Command = "pocketone"
