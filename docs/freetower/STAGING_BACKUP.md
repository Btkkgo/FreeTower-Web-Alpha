# FreeTower staging backup and restore

This runbook applies only to `FreeTower-Staging` / `staging`. A successful application redeploy is persistence evidence, not backup evidence. Backups contain wallet identity and gameplay data: keep them outside Git in access-controlled storage, never attach them to a report.

## Railway status

On 2026-10-08 the account remained HOBBY, with PostgreSQL 16.15 and a 500 MB Singapore volume. The backup schedule was empty; enabling DAILY returned `Not Authorized`. Automatic backup remains BLOCKED, with no plan upgrade or permission bypass. The user registered SSH key Btkkgo, and authorized SSH inspection and logical backup succeeded.

Two custom-format pg_dump archives were restored into separate local PostgreSQL 16.13 scratch databases using `--single-transaction --exit-on-error`, then the agent-created scratch databases were removed. Latest archive: `freetower-staging-20261008T050049Z.dump`, 9,517 bytes, SHA256 `6773eb7363fcf5a650a7148c28fcf09fb7a3b5034d08387c1dac711288bd0f40`. Restore verified one room, two buildings and zero production identities. Earlier restore also verified one staging identity. Archives are outside Git in `/Users/btkkgo/Documents/FreeTower-Staging-Backups` (directory 0700, archive 0600). These are manual verified logical backups, not an enabled recurring backup service. This drill did not replace the cloud database or claim a full application-level disaster recovery exercise.

Actual SSH dump uses the server's matching PostgreSQL client without exposing a database URL:

```sh
umask 077
npm exec --yes --package=@railway/cli -- railway ssh -s Postgres -e staging 'pg_dump -U postgres -d railway --format=custom --no-owner --no-privileges' > "$FT_BACKUP_FILE"
```

Check successful exit and `PGDMP` archive format before calling a dump valid. Keep backups and restore manifests private.

Railway exposes volume snapshots and, for eligible configurations, PostgreSQL point-in-time recovery. Check the service's Backups tab. Volume restore is limited to the same project/environment. The specific volume-backup reference says the original volume is retained unmounted and newer backups stay there; the general PostgreSQL guide still contains conflicting wording about newer backups. Verify the staged volumes and retained backups in the actual Dashboard before any explicitly approved recovery; this runbook does not assume deletion or retention has been observed. See [Railway backup guide](https://docs.railway.com/guides/postgres-backups-restores) and [volume backup reference](https://docs.railway.com/volumes/backups). Scheduled backups can incur storage usage; enabling must stay within the user's existing plan authorization.

## Logical backup

Use a PostgreSQL client version compatible with the deployed server. Obtain connection settings through authorized Railway tooling. Use a protected libpq service file and passfile outside the repository (permissions 0600); never put a password/URL on a command line, in shell history or in this document. Configure `freetower-staging` to the existing database and `freetower-restore-check` to a separately created empty scratch database. Protect transport with verified TLS or an authorized private tunnel. Never point the restore profile at staging itself.

Run in a private backup directory outside the repository; the commands intentionally do not provision services or credentials:

```sh
umask 077
FT_BACKUP_FILE="freetower-staging-$(date -u +%Y%m%dT%H%M%SZ).dump"
PGSERVICE=freetower-staging pg_dump --format=custom --no-owner --no-privileges --file="$FT_BACKUP_FILE"
shasum -a 256 "$FT_BACKUP_FILE" > "$FT_BACKUP_FILE.sha256"
pg_restore --list "$FT_BACKUP_FILE" > "$FT_BACKUP_FILE.manifest"
shasum -a 256 -c "$FT_BACKUP_FILE.sha256"
```

Do not treat `pg_restore --list` as a restore test. Capture UTC time, PostgreSQL version, dump size/hash and successful exit status without connection strings. At minimum the archive must include `sheep_rooms`, `freetower_identity_state`, `sheep_accounts` and `sheep_sessions`. Room JSON contains buildings, resources, fishing, farming, animals and private survival/inventory state; identity JSON contains wallet/character records. Dump files and manifests stay private.

## Restore drill

1. Arrange an empty, isolated scratch database with explicit capacity authorization; independently verify its host/database against the staging source. Do not overwrite the source, drop tables, or use `--clean`.
2. Restore with failure atomicity, stopping if the archive or schema is invalid:

```sh
PGSERVICE=freetower-restore-check pg_restore --single-transaction --exit-on-error --no-owner --no-privileges --dbname='service=freetower-restore-check' "$FT_BACKUP_FILE"
```

3. Verify schema and exact row counts in scratch. For comparison against the live source, quiesce writes during a defined acceptance checkpoint or export a repeatable snapshot: a changing live database is not an exact baseline.

```sh
PGSERVICE=freetower-restore-check psql -X --set=ON_ERROR_STOP=1 <<'SQL'
SELECT 'rooms' AS kind, count(*) FROM sheep_rooms
UNION ALL SELECT 'identity_state', count(*) FROM freetower_identity_state
UNION ALL SELECT 'legacy_accounts', count(*) FROM sheep_accounts
UNION ALL SELECT 'legacy_sessions', count(*) FROM sheep_sessions;
SELECT count(*) AS valid_identity_documents
FROM freetower_identity_state
WHERE jsonb_typeof(data->'wallets')='object'
  AND jsonb_typeof(data->'identities')='object'
  AND jsonb_typeof(data->'sessions')='object';
SQL
```

4. In an isolated staging-configured app, verify expected room/building/resource/farming/animal/survival values against private checkpoint evidence. Verify character IDs/environment are preserved. Do not print full wallet records. Boot-scoped cookies must be rejected after restart; reauthenticate with the real wallet and revalidate providers before confirming restored game entry.
5. Record restore elapsed time, backup age, table/count and gameplay checks, and PASS/FAIL separately. Keep the original staging service/database untouched. Delete scratch resources only through an explicitly authorized recoverable cleanup procedure.

## Incident recovery

First stop application writes and preserve current data. Review the restore target and recovery point with the owner. Restore to a new database, verify it, then change the application's `DATABASE_URL` reference and restart the app only. Retain the old database until recovery acceptance. No automatic database replacement or production cutover is authorized by this runbook.
