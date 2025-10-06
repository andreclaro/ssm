# SSM CLI Service Design Document

## Overview

This document outlines the design for a CLI service that manages AWS Systems Manager (SSM) instances across multiple AWS accounts and regions. The service provides functionality to discover, cache, and connect to EC2 instances using AWS Session Manager.

## Requirements

- **Multi-Account Support**: Work with multiple AWS profiles/accounts
- **Multi-Region Support**: Discover instances across all AWS regions
- **Local Caching**: Maintain a local database of discovered instances
- **Session Manager Integration**: Connect to instances via AWS SSM Session Manager
- **CLI Interface**: Simple command-line interface for listing and connecting

## Architecture

### Components

1. **CLI Interface** (`cmd/`)
   - Main entry point with cobra CLI framework
   - Command definitions for list and connect operations

2. **Core Service** (`internal/service/`)
   - Business logic for instance discovery and management
   - AWS SDK integration for SSM operations
   - Database operations for caching

3. **Database Layer** (`internal/storage/`)
   - Local storage abstraction
   - Instance metadata persistence
   - Query operations with filters

4. **AWS Integration** (`internal/aws/`)
   - AWS session management across profiles
   - SSM client operations
   - Region enumeration

5. **Configuration** (`internal/config/`)
   - AWS profile and region configuration
   - Database path configuration
   - CLI flag parsing

### Data Model

```go
type Instance struct {
    InstanceID   string    `json:"instance_id"`
    Name         string    `json:"name"`
    Region       string    `json:"region"`
    Profile      string    `json:"profile"`
    AccountID    string    `json:"account_id"`
    State        string    `json:"state"`
    LastSeen     time.Time `json:"last_seen"`
    Tags         []Tag     `json:"tags"`
}

type Tag struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}
```

### Database Design

**Format**: SQLite with GORM ORM
- **Pros**: ACID compliance, SQL queries, lightweight, cross-platform
- **Cons**: Slightly heavier than JSON, but provides better query capabilities
- **Location**: `~/.ssm-cli/database.db`

**Tables**:
- `instances`: Core instance metadata
- `tags`: Instance tags (many-to-many relationship)

### CLI Commands

#### `ssm <instance-name>`
Connect to a specific instance via Session Manager
- Resolves instance name to find matching instance across all profiles/regions
- Launches AWS SSM start-session command

#### `ssm --list [--region <region>] [--profile <profile>]`
List all discovered instances with optional filters
- Display format: Name, InstanceID, Region, Profile, AccountID
- Filters: region, profile
- Default: show all instances

#### `ssm --sync [--profile <profile>] [--region <region>]`
Manually trigger instance discovery
- Discover instances across specified profiles/regions
- Update local database with current state

### Discovery Process

1. **Profile Enumeration**: Load all available AWS profiles from `~/.aws/config`
2. **Region Enumeration**: Query all available AWS regions
3. **Instance Discovery**:
   - For each profile/region combination:
     - Create AWS session
     - Query EC2 instances via DescribeInstances API
     - Filter instances that have SSM agent installed (PlatformDetails contains "Windows" or "Linux")
     - Extract relevant metadata (ID, Name tag, State, etc.)
4. **Database Update**: Upsert instances in local database

### Session Management

- **AWS SDK v2**: Use AWS SDK for Go v2 for modern API support
- **Profile Handling**: Support multiple concurrent sessions for different profiles
- **Session Reuse**: Cache sessions per profile to avoid repeated authentication

### Error Handling

- **Network Errors**: Retry with exponential backoff for transient failures
- **Permission Errors**: Graceful handling of insufficient permissions per profile/region
- **Invalid Profiles**: Skip profiles that cannot authenticate
- **Partial Failures**: Continue discovery even if some regions/profiles fail

### Performance Considerations

- **Concurrent Discovery**: Use goroutines to discover instances across regions/profiles in parallel
- **Rate Limiting**: Respect AWS API rate limits with appropriate delays
- **Caching Strategy**: Cache discovery results with TTL (default: 1 hour)
- **Incremental Updates**: Only update changed instances to minimize database writes

### Security Considerations

- **Credential Management**: Rely on standard AWS credential providers (profiles, env vars, IAM roles)
- **No Credential Storage**: Never store AWS credentials in the database
- **Session Isolation**: Keep sessions isolated per profile to prevent cross-account contamination

### Configuration

**Config File**: `~/.ssm-cli/config.yaml`
```yaml
database:
  path: ~/.ssm-cli/database.db
  sync_interval: 168h

aws:
  default_profile: default
  default_region: us-east-1
  max_concurrent_sessions: 5

discovery:
  enabled: true
  ttl: 24h
  retry_attempts: 3
  retry_delay: 5s
```

### Dependencies

- **CLI Framework**: `github.com/spf13/cobra` - Command-line interface
- **AWS SDK**: `github.com/aws/aws-sdk-go-v2` - AWS API integration
- **Database**: `gorm.io/gorm` + `gorm.io/driver/sqlite` - ORM and SQLite driver
- **Configuration**: `github.com/spf13/viper` - Configuration management
- **Logging**: `github.com/sirupsen/logrus` - Structured logging

### Project Structure

```
ssm/
├── cmd/
│   └── root.go              # Main CLI entry point
├── internal/
│   ├── aws/                 # AWS SDK integration
│   │   ├── client.go        # AWS client management
│   │   └── ssm.go           # SSM operations
│   ├── config/              # Configuration management
│   │   └── config.go
│   ├── service/             # Core business logic
│   │   ├── discovery.go     # Instance discovery
│   │   ├── session.go       # Session management
│   │   └── instance.go      # Instance operations
│   └── storage/             # Database layer
│       ├── database.go      # Database connection
│       ├── instance.go      # Instance repository
│       └── migration.go     # Database migrations
├── docs/
│   └── design.md           # This design document
├── go.mod
├── go.sum
└── main.go
```

### Testing Strategy

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test AWS API interactions with mocked clients
- **CLI Tests**: Test command-line interface behavior
- **Database Tests**: Test database operations with in-memory SQLite

### Deployment & Distribution

- **Go Build**: Single binary with no external dependencies
- **Cross-Platform**: Build for Linux, macOS, Windows
- **Installation**: Simple binary copy or package manager distribution

### Future Enhancements

- **Auto-completion**: Shell completion for instance names
- **Instance Groups**: Logical grouping of instances
- **Bulk Operations**: Execute commands across multiple instances
- **Instance Monitoring**: Track instance health and connectivity
- **GUI Interface**: Optional web-based interface for instance management
