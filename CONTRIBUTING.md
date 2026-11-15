# Contributing to PocketOne

PocketOne is an open community fork of PocketBase that strongly encourages contributions. This document describes how to modify and work with the codebase.

- [Types of Contributions](#types-of-contributions)
- [LLM Policy](#llm-policy)
- [Prerequisites](#prerequisites)
- [Making changes in the Go code](#making-changes-in-the-go-code)
- [Making changes in the Superuser UI](#making-changes-in-the-superuser-ui)



## Types of Contributions

### PocketBase core changes

"Core" is upstream PocketBase's own code: every path outside `ext/`, `site/`, `examples/pocketone/`, `docs/`, `README.md`, `CONTRIBUTING.md`, `AGENTS.md`, `CLAUDE.md`, `.github/` and `.claude/`.

We are going to try our best to maintain upstream parity with Gani's great core work. Therefore, modifications to the core PocketBase code will undergo a more careful review before being accepted. Hooks to extend functionality are preferred over features so future merges from upstream are clean and easy. 

Every core change needs an independent audit: a reviewer who didn't write it (a human, or a fresh agent with no context from your session) checks that it is necessary, minimal and sensible, and the result goes in a `Core-Audit:` trailer on the commit. CI (`.github/workflows/core-audit.yml`) fails any commit that changes core without one. [AGENTS.md](AGENTS.md#every-core-change-gets-an-independent-audit-before-it-reaches-main) has the steps and the trailer format.

If you find a legitimate bug in the core code, by all means please submit a PR to fix it. We maintain a [separate list of upstream bugs](README.md#bugfixes) in case Gani wants to pick them up.

### Community Extension Contributions

This is where we think most people will spend their time. We have already added a bunch of hooks to PocketBase core to enable all kinds of extensions. The hook you need may already be there. If your extension requires a new hook in the core PocketBase code, that's fine.

The `/ext/*` folder is where your extension(s) live, and generally you own the code in your community extension's folder. It must pass a few CI checks (inspired by DefinitelyTyped) but is otherwise up to you in terms of code quality and approach. The standard extensions (the ones every `pocketone` binary ships with) also live in `ext/`, but changes to them get maintainer review.

## LLM Policy

Here, we take a major departure from PocketBase. LLMs are allowed and encouraged. You, the human, are responsible for the content of your PRs. Whether you use an LLM to achieve your desired result does not matter, but clean PRs do matter, especially if they touch the standard extensions or the core PocketBase code. 

## Prerequisites

- Go 1.27+ (for making changes in the Go code)
- Node 24+ (for making changes in the Superuser UI)

## Making changes in the Go code

PocketBase is distributed as a Go package, which means that in order to run the project you'll have to create a Go `main` program that imports the package.

The repository already includes such a program: `examples/pocketone` is the PocketOne standard binary (PocketBase plus the standard extensions), the one the prebuilt `pocketone` executables are built from. `examples/base/main.go` is upstream's plain PocketBase program, without any extensions.

So, let's assume that you already done some changes in the Go code and you want now to run them:

1. Navigate to `examples/pocketone`
2. Create a superuser first (otherwise the first `serve` opens the browser on the superuser install page): `go run . superuser upsert admin@example.com password`
3. Run `go run . serve`

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

Since the Superuser UI is just a client-side application, you need to have the PocketBase backend server also running in the background (either manually running `examples/pocketone` or download a prebuilt executable).

> By default, the Superuser UI is expecting the backend server to be started at `http://127.0.0.1:8090`, but you could change that by creating a new `ui/.env.development.local` file with `PB_BACKEND_URL = YOUR_ADDRESS` variable inside it.

Every change you make in the Superuser UI should be automatically reflected in the browser at `http://127.0.0.1:5173` without reloading the page.

When you are done with your changes, you have to build the Superuser UI with `npm run build` so that it can be embedded in the Go module.