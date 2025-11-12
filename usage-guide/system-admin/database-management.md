# Database Management

## Overview

This guide covers managing databases for the AIaaS platform, including migrations, backups, maintenance, and troubleshooting.

## Database Architecture

### Operational Database

- Stores core entities: organizations, users, API keys
- PostgreSQL database
- High availability configuration
- Regular backups required

### Analytics Database

- Stores usage events and analytics
- PostgreSQL database
- Partitioned by time
- Optimized for read-heavy workloads

## Database Migrations

### Running Migrations

1. Review migration files
2. Check current migration status
3. Run migrations in order
4. Verify migration success

```bash
make db-migrate-status
make db-migrate-up
```

### Creating Migrations

1. Create migration file with version number
2. Write up and down migrations
3. Test migrations locally
4. Review and commit

### Migration Best Practices

- Always provide down migrations
- Test migrations on staging first
- Backup before major migrations
- Monitor migration performance
- Document breaking changes

## Backups

### Backup Strategy

- **Full backups**: Daily
- **Incremental backups**: Hourly
- **Point-in-time recovery**: Enabled
- **Backup retention**: 30 days minimum

### Backup Procedures

1. Schedule automated backups
2. Verify backup integrity
3. Test restore procedures regularly
4. Store backups securely
5. Document backup locations

### Restore Procedures

1. Identify restore point
2. Stop affected services
3. Restore from backup
4. Verify data integrity
5. Restart services
6. Document incident

## Database Maintenance

### Regular Maintenance Tasks

- **Vacuum**: Reclaim storage space
- **Analyze**: Update query planner statistics
- **Index maintenance**: Rebuild fragmented indexes
- **Connection pool management**: Monitor and adjust

### Performance Tuning

- Monitor slow queries
- Optimize indexes
- Adjust connection pool sizes
- Configure query timeouts
- Review query plans

### Capacity Planning

- Monitor database growth
- Plan for storage increases
- Review partition strategies
- Optimize data retention policies

## Monitoring

### Key Metrics

- Database size and growth rate
- Connection pool usage
- Query performance
- Replication lag (if applicable)
- Backup success/failure

### Alerts

Configure alerts for:
- High connection usage
- Slow queries
- Replication lag
- Backup failures
- Disk space usage

## Troubleshooting

Common database issues:

- **Connection failures**: Check network and credentials
- **Slow queries**: Analyze query plans and indexes
- **Lock contention**: Review transaction isolation
- **Disk space**: Clean up old data or expand storage
- **Replication lag**: Check network and load

## Related Documentation

- [Database Schemas](../../specs/003-database-schemas/spec.md)
- [Migrations Runbook](../../docs/runbooks/migrations.md)
- [Database Guardrails](../../docs/troubleshooting/db-guardrails.md)

