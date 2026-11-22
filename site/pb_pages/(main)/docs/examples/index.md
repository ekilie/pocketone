---
title: Example apps
description: PocketOne apps you can try live and read end to end - Docs Chat, Bedtime Story and the Pages starter kits - what each does and how it's built.
---

# Example apps

Small, complete apps that show what a PocketOne app looks like. Each is a PocketOne app with no Go: a `pb_pages`
folder (and sometimes migrations and hooks) run by the stock `pocketone` binary. Their source is in the PocketOne
repository, ready to copy.

## Apps

Two apps built on [`$llm`](/docs/extensions/llm), PocketOne's cloud models, running live on PocketHost:

| App | Try it | Source |
|---|---|---|
| [**Docs Chat**](/docs/examples/docs-chat): ask about PocketOne and PocketBase, answered from both sets of docs | [learn.pocketone.dev](https://learn.pocketone.dev) | [`ext/llm/examples/chat`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/chat) |
| [**Bedtime Story**](/docs/examples/bedtime-story): one topic told slowly and in depth, read aloud with pictures and music | [sleep.pocketone.dev](https://sleep.pocketone.dev) | [`ext/llm/examples/bedtime`](https://github.com/pocketonehq/pocketone/tree/main/ext/llm/examples/bedtime) |

To run one yourself:

```bash
cd ext/llm/examples/bedtime        # or ext/llm/examples/chat
pocketone superuser upsert admin@example.com password --dir=pb_data
pocketone serve --dir=pb_data
```

Open the dashboard at `http://127.0.0.1:8090/_/`, add a provider in Settings → LLM (both apps default to Vultr models
under a provider named `vultr`), then open `http://127.0.0.1:8090`. To put your copy online, `pocketone link` your own
app and `pocketone deploy` ([Deploy and live sync](/docs/features/deploy)).

## Starters

Starting points for your own app, one command away:

```bash
pocketone pages create myapp --starter <kit>
```

| Starter | What it shows |
|---|---|
| [minimal](/docs/examples/starters#minimal) | A single page, the smallest PocketOne app |
| [mvp](/docs/examples/starters#mvp) | Classless MVP.css on plain semantic HTML |
| [auth](/docs/examples/starters#auth) | Password, anonymous, OTP and OAuth2 sign-in, verification, password reset and email change |
| [daisyui](/docs/examples/starters#daisyui) | Tailwind CSS with DaisyUI: a navbar and drawer |
| [daisyui-docs](/docs/examples/starters#daisyui-docs) | A docs site from Markdown pages |
| [htmx](/docs/examples/starters#htmx) | htmx with PocketBase realtime: a chat and a counter |
| [datastar](/docs/examples/starters#datastar) | Datastar with realtime: chat rooms and a counter |
| [tsx](/docs/examples/starters#tsx) | The same small app as TSX components |
