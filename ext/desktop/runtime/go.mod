module pocketone.dev/ext/desktop/runtime

go 1.27

replace (
	ganigeorgiev.com/pocketbase/pocketbase => ../../..
	pocketone.dev/ext/pages => ../../../ext/pages
	pocketone.dev/ext/s3redirect => ../../../ext/s3redirect
)

replace (
	pocketone.dev/ext/api-tokens => ../../../ext/api-tokens
	pocketone.dev/ext/deploy => ../../../ext/deploy
	pocketone.dev/ext/remotes => ../../../ext/remotes
)

replace (
	pocketone.dev/ext/mcp => ../../../ext/mcp
	pocketone.dev/ext/pitr => ../../../ext/pitr
)

replace pocketone.dev/ext/pocker => ../../../ext/pocker

replace pocketone.dev/ext/sandbox => ../../../ext/sandbox

replace pocketone.dev/ext/devusers => ../../../ext/devusers

replace pocketone.dev/ext/bulkmail => ../../../ext/bulkmail

replace pocketone.dev/ext/autotls => ../../../ext/autotls

replace pocketone.dev/ext/config => ../../../ext/config

replace pocketone.dev/ext/edge => ../../../ext/edge

replace pocketone.dev/ext/multisite => ../../../ext/multisite

replace pocketone.dev/ext/mailtrap => ../../../ext/mailtrap

replace pocketone.dev/ext/extkit => ../../../ext/extkit

replace pocketone.dev/ext/docker => ../../../ext/docker

replace pocketone.dev/ext/firewall => ../../../ext/firewall

replace pocketone.dev/ext/mothership => ../../../ext/mothership

replace pocketone.dev/ext/servertiming => ../../../ext/servertiming

replace pocketone.dev/ext/upgrade => ../../../ext/upgrade

replace pocketone.dev/ext/stats => ../../../ext/stats

replace pocketone.dev/ext/contracts => ../../../ext/contracts

replace pocketone.dev/ext/standard => ../../../ext/standard

replace pocketone.dev/ext/bundle => ../../../ext/bundle

replace pocketone.dev/ext/mailrelay => ../../../ext/mailrelay

replace pocketone.dev/ext/s3-fallback => ../../../ext/s3-fallback

replace pocketone.dev/ext/backups => ../../../ext/backups

replace pocketone.dev/ext/settings => ../../../ext/settings

replace pocketone.dev/ext/jsvm => ../../../ext/jsvm

replace pocketone.dev/ext/templates => ../../../ext/templates

replace pocketone.dev/ext/cache => ../../../ext/cache

replace pocketone.dev/ext/vacuum => ../../../ext/vacuum

require (
	ganigeorgiev.com/pocketbase/pocketbase v0.0.0-00010101000000-000000000000
	github.com/wailsapp/wails/v3 v3.1.0
	pocketone.dev/ext/bundle v0.0.0-00010101000000-000000000000
	pocketone.dev/ext/jsvm v0.0.0-00010101000000-000000000000
	pocketone.dev/ext/standard v0.0.0-00010101000000-000000000000
)

require (
	dario.cat/mergo v1.0.2 // indirect
	ganigeorgiev.com/fexpr v0.7.0 // indirect
	ganigeorgiev.com/pocketbase/dbx v1.13.0 // indirect
	ganigeorgiev.com/pocketbase/ozzo-validation/v4 v4.4.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/benbjohnson/litestream v0.5.18 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/domodwyer/mailyak/v3 v3.6.2 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/evanw/esbuild v0.28.2 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.16 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.2 // indirect
	github.com/go-git/go-git/v5 v5.19.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hablullah/go-hijri v1.0.2 // indirect
	github.com/hablullah/go-juliandays v1.0.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jalaali/go-jalaali v0.0.0-20210801064154-80525e88d958 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.4.0 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/magefile/mage v1.14.0 // indirect
	github.com/markusmobius/go-dateparser v1.2.4 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.23 // indirect
	github.com/pjbgf/sha1cd v0.7.0 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/psanford/sqlite3vfs v0.0.0-20260519004904-f9180fa2acc9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.1.0 // indirect
	github.com/superfly/ltx v0.5.2 // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	github.com/wasilibs/go-re2 v1.3.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/yuin/goldmark v1.8.7 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/image v0.48.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/libquickjs v0.13.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.62.0 // indirect
	pocketone.dev/ext/api-tokens v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/backups v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/bulkmail v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/cache v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/config v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/contracts v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/deploy v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/extkit v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/kv v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/llm v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/llm-chat v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/llm-search v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/mcp v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/multisite v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/pages v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/pitr v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/pocker v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/remotes v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/s3-fallback v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/s3redirect v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/sandbox v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/settings v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/templates v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/uicache v0.0.0-00010101000000-000000000000 // indirect
	pocketone.dev/ext/wasm v0.0.0-00010101000000-000000000000 // indirect
)

replace pocketone.dev/ext/wasm => ../../../ext/wasm

replace pocketone.dev/ext/llm => ../../llm

replace pocketone.dev/ext/uicache => ../../../ext/uicache

replace pocketone.dev/ext/llm-chat => ../../llm-chat

replace pocketone.dev/ext/kv => ../../kv

replace pocketone.dev/ext/llm-search => ../../llm-search
