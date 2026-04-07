# Daptin CLI

CLI client for [Daptin](https://github.com/daptin/daptin) — the headless CMS and API server.

All Daptin entities are accessed uniformly via CRUD commands. All Daptin actions (signin, signup, upload, export, etc.) are executed uniformly via `execute`. No special-case commands.

## Install

### Homebrew (macOS / Linux)

```bash
brew install daptin/tap/daptin-cli
```

### Scoop (Windows)

```powershell
scoop bucket add daptin https://github.com/daptin/scoop-bucket
scoop install daptin-cli
```

### Debian / Ubuntu

Download the `.deb` from the [latest release](https://github.com/daptin/daptin-cli/releases/latest):

```bash
sudo dpkg -i daptin-cli_*.deb
```

### RPM (Fedora / RHEL)

```bash
sudo rpm -i daptin-cli_*.rpm
```

### Go

```bash
go install github.com/daptin/daptin-cli@latest
```

### Binary download

Grab a binary from the [releases page](https://github.com/daptin/daptin-cli/releases/latest) for linux, macOS, or Windows (amd64 / arm64).

## Quick Start

```bash
# Point to your Daptin server
daptin-cli context add myserver http://localhost:6336
daptin-cli context set myserver

# Sign in (signin is just an action like any other)
daptin-cli execute user_account signin email=admin@example.com password=secret

# List tables
daptin-cli list --columns table_name,is_top_level world

# List rows
daptin-cli list --columns name,email --page-size 20 user_account
```

## Context Management

Contexts store server endpoints and auth tokens in `~/.daptin/config.yaml`.

```bash
daptin-cli context add prod https://api.example.com
daptin-cli context add local http://localhost:6336
daptin-cli context set prod
daptin-cli context list
```

Override per-command with `--endpoint`:

```bash
daptin-cli --endpoint http://localhost:6336 list world
```

## CRUD

### List rows

```bash
daptin-cli list <entity> [flags]
```

Flags: `--columns`, `--page-size`, `--page`, `--sort`, `--filter`, `--include`

```bash
# List with column selection and pagination
daptin-cli list --columns table_name,reference_id --page-size 50 world

# Sort descending by created_at
daptin-cli list --sort -created_at --page-size 10 document

# Filter
daptin-cli list --filter "status is active" task
daptin-cli list --filter "table_name like %doc%" world
daptin-cli list --filter "name is admin;email contains example" user_account

# Include relations
daptin-cli list --include user_account_id document
```

### Get a single row

```bash
daptin-cli get <entity> <reference_id> [--columns col1,col2]
```

```bash
daptin-cli get world 019228bb-a7cd-773b-a465-c92d7c54d956
daptin-cli get --columns table_name,is_top_level world 019228bb-a7cd-773b-a465-c92d7c54d956
```

### Create, Update, Delete

```bash
# Create with key=value pairs
daptin-cli create document document_name=report.pdf document_extension=pdf

# Create with JSON
daptin-cli create document '{"document_name":"report.pdf","document_extension":"pdf"}'

# Update
daptin-cli update document <reference_id> document_name=updated.pdf

# Delete
daptin-cli delete document <reference_id>
```

### Traverse relationships

```bash
daptin-cli related <entity> <reference_id> <relation_column>
daptin-cli related document <ref_id> user_account_id
```

## Actions

All Daptin actions — built-in or custom — are executed with `execute`. There are no special commands for signin, signup, upload, etc.

```bash
daptin-cli execute <entity> <action_name> [key=val ...]
```

### Authentication

```bash
# Sign in
daptin-cli execute user_account signin email=admin@example.com password=secret

# Sign up
daptin-cli execute user_account signup name=Alice email=alice@example.com password=secret passwordConfirm=secret

# Sign in with 2FA
daptin-cli execute user_account verify_otp email=admin@example.com mobile_number=+1234567890 otp=123456
```

On successful signin, the token is automatically saved to the active context.

### Other actions

```bash
# Generate random data
daptin-cli execute world generate_random_data table_name=products count=100

# Export data
daptin-cli execute world export_data table_name=document format=json

# Upload file to cloud store
daptin-cli execute cloud_store upload_file --reference-id <store_id> path=/uploads

# Download system schema
daptin-cli execute world download_system_schema
```

### Interactive mode

Prompt for fields based on the action's InFields schema:

```bash
daptin-cli execute user_account signin --interactive
# > Email: admin@example.com
# > Password: ****
```

Password fields are automatically masked.

### Instance actions

For actions that require an entity instance, pass `--reference-id`:

```bash
daptin-cli execute cloud_store upload_file --reference-id <cloud_store_id> path=/docs
```

## Describe

### Table schema

```bash
daptin-cli describe table document
```

Shows columns (name + type) and available actions.

### Action schema

```bash
daptin-cli describe action document createDocument
```

Shows the action's InFields (the parameters it accepts).

## Output

Table (default) or JSON:

```bash
daptin-cli --output table list world
daptin-cli --output json list world
```

## Filter Syntax

Filters are semicolon-separated `<column> <operator> <value>` expressions:

```bash
--filter "name is alice"
--filter "status is active;role is admin"
--filter "name like %ali%"
--filter "count more than 5"
--filter "active is true"
--filter "notes is empty"
```

Operators: `is`, `like`, `ilike`, `contains`, `neq`, `gt`, `lt`, `more than`, `less than`, `begins with`, `ends with`, `in`, `is true`, `is false`, `is empty`, `is not`, `fuzzy`

Use `%` wildcards with `like` for partial matching. Raw JSON is also accepted:

```bash
--filter '[{"column":"name","operator":"like","value":"%ali%"}]'
```

## Global Flags

```
--config FILE, -c    Config file (default: ~/.daptin/config.yaml)
--output, -o         Output format: table or json (default: table)
--endpoint           Server endpoint (default: http://localhost:6336)
--debug              Enable debug output
```

## WebSocket

Real-time pub/sub via Daptin's `/live` endpoint.

```bash
# Stream all events
daptin-cli ws listen

# Subscribe to topics
daptin-cli ws subscribe user_account
daptin-cli ws subscribe user_account document order

# Publish a message
daptin-cli ws publish chat-room-1 '{"text":"hello"}'

# Ping
daptin-cli ws ping

# Topic management
daptin-cli ws topic create chat-room-1
daptin-cli ws topic delete chat-room-1
daptin-cli ws topic permission chat-room-1
daptin-cli ws topic permission chat-room-1 --set 2097151

# Cross-node verification
daptin-cli ws verify --endpoints http://node1:6336,http://node2:6336
```

## Environment Variables

```
DAPTIN_CLI_CONFIG    Config file path
DAPTIN_ENDPOINT      Server endpoint
DAPTIN_CLI_OUTPUT    Output format
```
