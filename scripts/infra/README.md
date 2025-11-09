# Infrastructure Automation Scripts

Helpers for orchestrating Terraform and supporting workflows (plan, apply, rollback, drift detection). Scripts added in later phases should be invoked via Make targets to keep tooling consistent.

- `state-backup.sh` – pulls terraform state for an environment and writes it to `infra/terraform/backups/` (uploads to S3 if `STATE_BACKUP_BUCKET` is set).
