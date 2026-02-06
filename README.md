# Daptin CLI

Command-line interface for [Daptin](https://github.com/daptin/daptin) Backend-as-a-Service.

Built with TypeScript, inspired by [daptin-js-client](https://github.com/artpar/daptin-js-client).

## Installation

```bash
npm install -g daptin-cli
```

Or build from source:

```bash
git clone https://github.com/daptin/daptin-cli.git
cd daptin-cli
npm install
npm run build
npm link
```

## Quick Start

```bash
# Add a Daptin server
daptin-cli config add local http://localhost:6336

# Sign in
daptin-cli signin admin@example.com

# List all tables
daptin-cli tables

# List records from a table
daptin-cli list user_account --page-size 20

# Get a single record
daptin-cli get user_account <reference_id>
```

## Configuration

The CLI stores configuration in `~/.daptin/config.yaml`. You can manage multiple server contexts:

```bash
# Add servers
daptin-cli config add local http://localhost:6336
daptin-cli config add production https://api.example.com

# Switch active server
daptin-cli config use production

# List all servers
daptin-cli config list

# Show current context
daptin-cli config show

# Remove a server
daptin-cli config remove local
```

Override the endpoint on any command with `--endpoint`:

```bash
daptin-cli --endpoint http://localhost:6336 tables
```

## Global Options

| Flag | Description | Default |
|------|-------------|---------|
| `-e, --endpoint <url>` | Daptin server endpoint | from config |
| `-c, --config <path>` | Config file path | `~/.daptin/config.yaml` |
| `-o, --output <format>` | Output format (`table`, `json`) | `table` |
| `-t, --token <token>` | Auth token | from config |
| `--debug` | Enable debug output | `false` |

## Commands

### Authentication

```bash
# Create a new account
daptin-cli signup user@example.com

# Sign in (prompts for password)
daptin-cli signin user@example.com

# Sign in with 2FA
daptin-cli signin-2fa user@example.com

# Show current user info
daptin-cli whoami
```

### Schema Discovery

```bash
# List all tables
daptin-cli tables
daptin-cli tables --columns table_name,is_top_level,default_permission

# Describe a table (columns, relations, actions)
daptin-cli describe user_account
daptin-cli describe product --columns ColumnName,ColumnType,DataType,IsNullable
```

### CRUD Operations

```bash
# List records
daptin-cli list product
daptin-cli list product --page-size 50 --page 2
daptin-cli list product --columns name,price,reference_id
daptin-cli list product --sort -price,name
daptin-cli list product --include category_id

# Get a single record
daptin-cli get product <reference_id>
daptin-cli get product <reference_id> --columns name,price

# Create a record
daptin-cli create product --data '{"name": "Widget", "price": 9.99}'

# Update a record
daptin-cli update product <reference_id> --data '{"price": 12.99}'

# Delete a record
daptin-cli delete product <reference_id>

# Query relationships
daptin-cli relation customer <reference_id> order_id --page-size 20
```

### Actions

```bash
# List all actions
daptin-cli actions

# List actions for a specific table
daptin-cli actions user_account

# Show action schema (input/output fields)
daptin-cli action-describe user_account signin

# Execute an action
daptin-cli execute user_account generate_jwt_token --data '{"email": "user@example.com"}'

# Execute an instance-level action
daptin-cli execute order mark_shipped --id <order_reference_id> --data '{"tracking_number": "ABC123"}'
```

### Aggregation

```bash
# Count records
daptin-cli aggregate product --columns count

# Sum and average
daptin-cli aggregate product --columns "count,sum(price),avg(price)"

# Group by
daptin-cli aggregate order --columns "status,count,sum(total)" --group status

# With filters
daptin-cli aggregate product --columns "count,avg(price)" --filter "gt(price,100)"

# With limit and sort
daptin-cli aggregate order --columns "status,count" --group status --sort "-count" --limit 10
```

### Output Formats

```bash
# Table output (default)
daptin-cli list product --output table

# JSON output
daptin-cli list product --output json

# JSON output piped to jq
daptin-cli list product -o json | jq '.[].name'
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DAPTIN_CLI_CONFIG` | Path to config file |
| `DAPTIN_ENDPOINT` | Default server endpoint |

## Development

```bash
# Install dependencies
npm install

# Run in development mode
npx tsx src/index.ts tables

# Type check
npm run typecheck

# Build
npm run build
```

## License

MIT
