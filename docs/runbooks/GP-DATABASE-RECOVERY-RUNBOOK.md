# GreenPass database backup and recovery runbook

This runbook applies only to the GreenPass PostgreSQL workload in namespace
`gp`, database `gp`. It does not authorize access to, or changes in,
`open-im-local`.

## Backup

Create a directory outside the repository for recovery artifacts, then run:

```powershell
./scripts/dev/backup-gp-database.ps1 -OutputPath D:\gp-backups\gp-before-upgrade.sql
```

The script requires exactly one `app=gp-postgres` pod in `gp`, reads its
credential only inside the Pod, and refuses to overwrite an existing file.
Record the Git commit, Helm revision, image digests, migration version and the
artifact checksum with the backup.

## Restore rehearsal

Restore is destructive only to the dedicated `gp` database. It drops and
recreates that database, then imports the supplied SQL file. It requires the
literal confirmation below and never addresses another namespace or database.

```powershell
./scripts/dev/restore-gp-database.ps1 `
  -BackupPath D:\gp-backups\gp-before-upgrade.sql `
  -ConfirmRestore RESTORE_GP_DATABASE
```

After restore, use the normal Helm-owned migration path rather than running
application migrations by hand. Verify `schema_migrations`, `GET /api/readyz`,
and the GP smoke flow. A rehearsal is not a production RPO/RTO claim; those
targets remain dependent on the approved backup store and recovery objective.
