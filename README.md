# OTP Service

An OTP backend built with Go, PostgreSQL, and Redis. The project is being
developed incrementally as a production-minded modular monolith.

## Current status

- Phase 1: HTTP server, routing, and graceful shutdown — complete.
- Phase 2.1: PostgreSQL Docker Compose setup — complete.
- Phase 2.2: OTP database design and SQL schema — complete.
- Phase 2.3: Versioned database migrations — complete.
- Phase 2.4: pgx connection pool — complete.
- Phase 2.5: PostgreSQL OTP repository — complete.
- Phase 2.6: Secure OTP generation — complete.

## Database schema

The Phase 2 OTP schema stores only hashed OTP values and models expiration and
single use explicitly. See [the database design](docs/database-design.md) and
the canonical [SQL schema](db/schema.sql).

The initial schema is versioned in [migrations](migrations), with an explicit
Docker workflow documented in [database migrations](docs/migrations.md).

## Local run

Start PostgreSQL and apply migrations first:

```sh
docker compose up -d postgres
make migration-up
```

Then provide a database URL and start the server:

```sh
export DATABASE_URL='postgres://otp_user:otp_password@localhost:5432/otp_db?sslmode=disable'
go run ./cmd/server
```

The server verifies PostgreSQL connectivity before accepting requests and
closes its connection pool during graceful shutdown. The optional pool settings
are `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_MAX_CONN_LIFETIME`,
`DB_MAX_CONN_IDLE_TIME`, and `DB_HEALTH_CHECK_PERIOD`.

## Architecture

OTP persistence is isolated behind the `internal/otp.Repository` interface.
The PostgreSQL implementation stores only the supplied OTP hash and returns the
database-generated record. OTP generation uses `crypto/rand` to produce numeric
codes without predictable randomness. Hashing, sending, and verification are
implemented in later phases.
