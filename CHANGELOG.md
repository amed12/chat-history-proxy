# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `cmd/chat-history-proxy` — read-only Qiscus chat-history proxy (no webhook, no
  database), generic for any client running Qiscus in sessional mode. See
  [`docs/CHAT_HISTORY_PROXY.md`](./docs/CHAT_HISTORY_PROXY.md).
- `internal/proxy/config` — environment-only configuration loader, fails fast on
  missing required credentials.
- `internal/proxy/qiscus` — admin REST client for `get_user_rooms`/`load_comments`,
  including per-room `started_at`/`last_message`/`topic` derivation.
- `internal/proxy/cache` — generic in-memory TTL cache (no external dependency),
  with a `?fresh=1` bypass for right-after-creating-a-room reads.
- `internal/proxy/middleware` — RS256 JWT verification.
- `internal/proxy/handler` — `/api/v1/sessions` and
  `/api/v1/sessions/{room_id}/messages`, with room-ownership verification.
- `cmd/devtools/gen-dev-jwt` — local-only dev JWT generator for testing without a
  real client auth backend.
