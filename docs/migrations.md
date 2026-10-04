# Database Migrations

Database changes are versioned with [golang-migrate](https://github.com/golang-migrate/migrate).
The migration image is used only as an explicit development command; starting
PostgreSQL does not automatically change its schema.

## Setup

Copy `.env.example` to `.env` if you need to change the local PostgreSQL
credentials. Do not commit `.env` files. Passwords that contain URL-reserved
characters must be percent-encoded when invoking `make` migration targets.

Start PostgreSQL, then run the pending migrations:

```sh
docker compose up -d postgres
make migration-up
```

Useful commands:

```sh
make migration-version # show the applied migration version
make migration-down    # revert exactly one migration
```

## Creating migrations

Each schema change requires a sequential, descriptive pair of files in
`migrations/`:

```text
000002_add_example.up.sql
000002_add_example.down.sql
```

Test both directions locally before committing. Never edit a migration that
has been applied to a shared environment; add a new migration instead.
