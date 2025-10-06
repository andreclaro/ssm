## Architecture

The CLI consists of several components:

- CLI Layer: Command-line interface using Cobra
- Service Layer: Business logic for instance discovery and management
- Storage Layer: SQLite database for caching instance metadata
- AWS Layer: AWS SDK integration for EC2 and SSM operations
