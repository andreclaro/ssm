## Configuration

The CLI works out of the box without any configuration files. On first run, it will:

1. Discover AWS profiles from `~/.aws/config`
2. Run an interactive setup to choose profiles and regions
3. Create a SQLite database at `~/.ssm/database.db`
4. Run initial sync to discover EC2 instances

Re-run setup anytime:

```bash
ssm setup
```

### Data directory

All application data is stored in `~/.ssm/` (database, cache, etc.)

### AWS configuration

The CLI automatically discovers available AWS profiles and supports:

- AWS CLI configuration files (`~/.aws/config`, `~/.aws/credentials`)
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, ...)
- IAM roles and instance profiles
- SSO-based authentication

It scans `~/.aws/config` and `~/.aws/credentials` to discover configured profiles.

### Configuration options

Edit `~/.ssm/config.yaml` to customize behavior:

```yaml
database:
  path: ~/.ssm/database.db

aws:
  max_concurrent_sessions: 5

discovery:
  ttl: 24h
```


