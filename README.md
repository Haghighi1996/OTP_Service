# OTP Service

An OTP backend built with Go, PostgreSQL, and Redis. The project is being
developed incrementally as a production-minded modular monolith.

## Current status

- Phase 1: HTTP server, routing, and graceful shutdown — complete.
- Phase 2.1: PostgreSQL Docker Compose setup — complete.
- Phase 2.2: OTP database design and SQL schema — complete.
- Phase 2.3: Versioned database migrations — complete.

## Database schema

The Phase 2 OTP schema stores only hashed OTP values and models expiration and
single use explicitly. See [the database design](docs/database-design.md) and
the canonical [SQL schema](db/schema.sql).

The initial schema is versioned in [migrations](migrations), with an explicit
Docker workflow documented in [database migrations](docs/migrations.md). The
Go PostgreSQL connection layer is introduced in a later phase.
