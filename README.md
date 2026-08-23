# Illarin

A catalog for AI roleplay assets: characters, lorebooks, presets, themes, and DLC.

Run `make` for the command list. On a fresh development clone, run `make setup`.

The production application is one Docker Compose project on an OVH VPS. Pull
requests are checked before immutable Go and Next.js images can be published;
production deployment remains a manual, health-gated action. Start with
[`ops/README.md`](ops/README.md) for setup, deployment, monitoring, and recovery.
