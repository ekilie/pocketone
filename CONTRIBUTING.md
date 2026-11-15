# Contributing to PocketOne

PocketOne is an open community fork of PocketBase that strongly encourages contributions. This document describes how to modify and work with the codebase.

- [Types of Contributions](#types-of-contributions)
- [LLM Policy](#llm-policy)
- [Prerequisites](#prerequisites)
- [Making changes in the Go code](#making-changes-in-the-go-code)
- [Making changes in the Superuser UI](#making-changes-in-the-superuser-ui)



## Types of Contributions

### Core PocketBase Contributions

We are going to try our best to maintain upstream parity with Gani's great core work. Therefore, modifications to the core PocketBase code will undergo a more careful review before being accepted. Hooks to extend functionality are preferred over features so future merges from upstream are clean and easy. 

If you find a legitimate bug in the core code, by all means please submit a PR to fix it. We maintain a separate list of upstream bugs in case Gani wants to pick them up.

### Community Extension Contributions

This is where we think most people will spend their time. We have already added a bunch of hooks to PocketBase core to enable all kinds of extensions. The hook you need may already be there. If your extenion requires a new hook in the core PocketBase code, that's fine.

The `/ext/*` folder is where your extension(s) live, and generally you own the code in your folder. It must pass a few CI checks (inspired by DefinitelyTyped) but is otherwise up to you in terms of code quality and approach.

## LLM Policy

Here, we take a major departure from PocketBase. LLM's are allowed and encouraged. You, the human, are responsible for the content of your PR's. Whether you use an LLM to acheive your desired result does not matter, but clean PR's do matter especially if they touch the builtin PocketOne extensions or the core PocketBase code. 

## Prerequisites

- Go 1.27+ (for making changes in the Go code)
- Node 24+ (for making changes in the Superuser UI)

## Making changes in the Go code

PocketBase is distributed as a Go package, which means that in order to run the project you'll have to create a Go `main` program that imports the package.

The repository already includes such program, located in `examples/base/main.go`, that is also used for the prebuilt executables.

So, let's assume that you already done some changes in the PocketBase Go code and you want now to run them:

1. Navigate to `examples/base`
2. Run `go run main.go serve`

This will start a web server on `http://127.0.0.1:8090` with the embedded prebuilt Superuser UI from `ui/dist`. And that's it!

To run the tests, you can execute (while in the root project directory):

  ```sh
  go test ./...

  # or using the Makefile
  make test
  ```

## Making changes in the Superuser UI

PocketBase Superuser UI is a single-page application (SPA) built with Svelte and Vite.

To start the Superuser UI:

1. Navigate to the `ui` project directory
2. Run `npm install` to install the node dependencies
3. Start vite's dev server
   ```sh
   npm run dev
   ```

You could open the browser and access the running Superuser UI at `http://127.0.0.1:5173`.

Since the Superuser UI is just a client-side application, you need to have the PocketBase backend server also running in the background (either manually running the `examples/base/main.go` or download a prebuilt executable).

> By default, the Superuser UI is expecting the backend server to be started at `http://127.0.0.1:8090`, but you could change that by creating a new `ui/.env.development.local` file with `PB_BACKEND_URL = YOUR_ADDRESS` variable inside it.

Every change you make in the Superuser UI should be automatically reflected in the browser at `http://127.0.0.1:5173` without reloading the page.

When you are done with your changes, you have to build the Superuser UI with `npm run build` so that it can be embedded in the Go module.