# Migrations

SQL migration files for PayFlow, managed by [golang-migrate](https://github.com/golang-migrate/migrate).

## File naming convention

```
<version>_<description>.up.sql    — apply migration
<version>_<description>.down.sql  — revert migration
```

## Migrations are added in:

| Migration | Phase | Description |
|---|---|---|
| `001` | Phase 2 | Create `payments` table |
| `002` | Phase 2 | Create `refunds` table |
| `003` | Phase 5 | Create `outbox_events` table |
| `004` | Phase 5 | Create `webhook_deliveries` table |
| `005` | Phase 6 | Create `ledger_entries` table |

## Running migrations

```bash
bash scripts/migrate.sh up
```

See [`scripts/migrate.sh`](../scripts/migrate.sh) for full usage.
