# SSM CLI

A fast CLI to discover and connect to servers via AWS Systems Manager (SSM) across multiple AWS accounts and regions.

## Quick start

```bash
go install github.com/andreclaro/ssm@latest

# or build locally:
git clone https://github.com/andreclaro/ssm && cd ssm && go build -o ssm .
```

```bash
ssm sync # discover instances
ssm list # list available instances
ssm my-db # connect via SSM Session Manager
ssm my-server -L 8888:80 # port forward local 8888 to remote 80
```

## Features

- Multi-account and multi-region discovery
- Local cache in SQLite (`~/.ssm/database.db`)
- One-command SSM connections
- Shell completion for instance names

## Documentation

- [Installation](docs/installation.md)
- [Usage](docs/usage.md)
- [Shell completion](docs/completion.md)
- [Configuration](docs/configuration.md)
- [Architecture](docs/architecture.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Design](docs/design.md)

## License

MIT — see [LICENSE](LICENSE).
