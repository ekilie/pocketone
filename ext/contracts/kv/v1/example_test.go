package kvv1_test

import (
	"fmt"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	kvv1 "pocketone.dev/ext/contracts/kv/v1"
)

// app is the app the examples run in: in your code, the core.App your extension or main.go has.
var app core.App

func ExampleGet() {
	var settings struct{ Theme string }
	found, err := kvv1.Get(app, "myapp/settings", &settings)
	if err != nil {
		return
	}
	if !found {
		settings.Theme = "light"
	}
	fmt.Println(settings.Theme)
}

func ExampleSet() {
	err := kvv1.Set(app, "myapp/settings", map[string]any{"theme": "dark"})
	if err != nil {
		return
	}
}

func ExampleDelete() {
	_ = kvv1.Delete(app, "myapp/settings")
}

func ExampleList() {
	entries, err := kvv1.List(app, "myapp/")
	if err != nil {
		return
	}
	for _, e := range entries {
		fmt.Println(e.Name, e.Updated)
	}
}
