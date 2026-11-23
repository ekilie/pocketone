package main

import (
	"strings"
	"testing"
)

const jsSample = `/** One entry. */
type Entry = { name: string }

type API = {
  /** Reads one. */
  get(name: string): any
  /** Grouped calls. */
  sub: {
    /** Does it later. */
    laterAsync(opts: { a: number }): Promise<{
      done: boolean
    }>
  }
}

/**
 * The thing, as $thing.
 *
 * More words that aren't the summary.
 */
declare var $thing: API & {
  /** Narrows it: see {@link Entry}. */
  for(e: any): API
}

/** Modules. */
declare namespace $mods {
  namespace run {
    /** Runs one. */
    function async<T = any>(name: string): Promise<T>
  }
  /** Every module. */
  function list(): string[]
}

/** Overloaded. */
declare function onThing(fn: () => void): void
declare function onThing(id: string, fn: () => void): void
`

func TestJSGlobals(t *testing.T) {
	gs := jsGlobals(jsSample)
	var names []string
	for _, g := range gs {
		names = append(names, g.Name)
	}
	if got := strings.Join(names, ","); got != "$thing,$mods,onThing" {
		t.Fatalf("globals: %s", got)
	}
	if gs[0].Doc != "The thing, as $thing." {
		t.Fatalf("summary: %q", gs[0].Doc)
	}
	ms := jsGlobalMembers(jsSample)
	var sigs []string
	for _, m := range ms["$thing"] {
		sigs = append(sigs, m.Sig+" = "+m.Doc)
	}
	if !ms["$thing"][1].Group || ms["$thing"][0].Group {
		t.Fatalf("groups: %+v", ms["$thing"])
	}
	want := []string{
		"$thing.get(name: string) = Reads one.",
		"$thing.sub = Grouped calls.",
		"$thing.sub.laterAsync(opts: { a: number }) = Does it later.",
		"$thing.for(e: any) = Narrows it: see `Entry`.",
	}
	if strings.Join(sigs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("$thing members:\n%s", strings.Join(sigs, "\n"))
	}
	var mods []string
	for _, m := range ms["$mods"] {
		mods = append(mods, m.Sig)
	}
	if strings.Join(mods, ",") != "$mods.run,$mods.run.async(name: string),$mods.list()" {
		t.Fatalf("$mods members: %v", mods)
	}
}
