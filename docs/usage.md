## Usage

### Connect to an instance

```bash
# Connect to an instance by name
ssm my-instance-name
# The CLI searches across configured profiles and regions
```

### List instances

```bash
ssm list            # Default view (SSM-managed Online)
ssm list --all      # Show all instances
ssm list --region us-east-1
ssm list --profile myprofile
ssm list --profile dev --region us-west-2
```

### Sync instances

```bash
ssm sync                  # Sync all instances (enabled regions)
ssm sync --profile prod   # Sync for a specific profile
ssm sync --region eu-west-1
```

### Update regions

```bash
ssm update-regions        # Interactive region enable/disable
```

### Global options

```bash
ssm --verbose list
ssm --profile myprofile list
ssm --region us-east-1 list
ssm --config /path/to/config.yaml list
```

### Examples

#### Basic workflow

```bash
ssm sync
ssm list
ssm my-web-server
```

#### Multiple accounts

```bash
ssm sync
ssm list --profile production
ssm list --profile staging
ssm prod-web-01
ssm staging-db-01
```
