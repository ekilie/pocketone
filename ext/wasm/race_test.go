//go:build race

package wasm_test

// raceEnabled: the tests run with -race, under which wazero compiles a module about ten times slower.
const raceEnabled = true
