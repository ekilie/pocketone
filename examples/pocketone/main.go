// Command pocketone is the standard PocketOne binary (package binary). It's its own module so core's go.mod stays
// upstream's; builds on top of PocketOne (PocketHost's `pockethost`) call binary.Main with their own commands.
package main

import "pocketone.dev/examples/pocketone/binary"

func main() {
	binary.Main(binary.Options{})
}
