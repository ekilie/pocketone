module github.com/pockethost/pocketone/ext/tokens

go 1.27

require (
	ganigeorgiev.com/pocketbase/dbx v1.13.0
	ganigeorgiev.com/pocketbase/pocketbase v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/contracts v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/devusers v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/extkit v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/jsvm v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/remotes v0.0.0-00010101000000-000000000000
	github.com/pockethost/pocketone/ext/templates v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.10.2
)

require (
	ganigeorgiev.com/fexpr v0.7.0 // indirect
	ganigeorgiev.com/pocketbase/ozzo-validation/v4 v4.4.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/domodwyer/mailyak/v3 v3.6.2 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/evanw/esbuild v0.28.2 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.16 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.1.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/image v0.48.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/libquickjs v0.13.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.62.0 // indirect
)

replace ganigeorgiev.com/pocketbase/pocketbase => ../..

replace github.com/pockethost/pocketone/ext/devusers => ../devusers

replace github.com/pockethost/pocketone/ext/pocker => ../pocker

replace github.com/pockethost/pocketone/ext/sandbox => ../sandbox

replace github.com/pockethost/pocketone/ext/edge => ../edge

replace github.com/pockethost/pocketone/ext/multisite => ../multisite

replace github.com/pockethost/pocketone/ext/extkit => ../extkit

replace github.com/pockethost/pocketone/ext/config => ../config

replace github.com/pockethost/pocketone/ext/contracts => ../contracts

replace github.com/pockethost/pocketone/ext/pages => ../pages

replace github.com/pockethost/pocketone/ext/remotes => ../remotes

replace github.com/pockethost/pocketone/ext/jsvm => ../jsvm

replace github.com/pockethost/pocketone/ext/templates => ../templates
