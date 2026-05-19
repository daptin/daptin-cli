# Daptin CLI

CLI client for [Daptin](https://github.com/daptin/daptin) — the headless CMS and API server.

All Daptin entities are accessed uniformly via CRUD commands, and all Daptin actions can be executed uniformly via `execute`. Common workflows such as cloud storage and file asset uploads also have ergonomic wrapper commands.

## Discovery

The CLI is self-describing. Start with:

```bash
daptin-cli --help
daptin-cli list --help
daptin-cli execute --help
daptin-cli describe action --help
daptin-cli oauth --help
daptin-cli integration --help
daptin-cli storage --help
daptin-cli asset --help
```

For any Daptin action, use `describe action` before `execute` to see whether the action needs an instance reference id and which input fields it accepts:

```bash
daptin-cli describe action integration install_integration
daptin-cli execute integration install_integration --reference-id <integration_reference_id>
```

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

# Sign in (signin is an action like any other)
daptin-cli execute user_account signin email=admin@example.com password=secret

# List tables
daptin-cli list --columns table_name,is_top_level world

# List rows
daptin-cli list --columns name,email --page-size 20 user_account

# Discover action requirements
daptin-cli describe action integration install_integration
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
daptin-cli list --filter name=administrators usergroup
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

All Daptin actions — built-in or custom — can be executed with `execute`.

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

Use `describe action` when you are unsure:

```bash
daptin-cli describe action cloud_store upload_file
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

Shows whether the action is instance-bound, whether `--reference-id` is required, the action's InFields, and an example `execute` command.

## Table Defaults

Use `table defaults` to inspect and update schema-level defaults before creating
rows in setup scripts:

```bash
daptin-cli table defaults get oauth_connect
daptin-cli table defaults set oauth_connect --permission 1618275
daptin-cli table defaults group add oauth_connect users --permission 1618275
daptin-cli table defaults ensure oauth_connect --permission 1618275 --group users:1618275
```

These commands update the table's `world_schema_json`; restart Daptin or run the
supported reload flow before relying on changed defaults in a long-running
server process.

## OAuth And Integrations

OAuth provider setup and OpenAPI integration workflows have first-class wrappers. The generic `create`, `list`, `describe action`, and `execute` commands still work, but these commands keep the common lifecycle discoverable and avoid passing large specs as shell arguments.

```bash
# Register a Daptin OAuth provider client app
export APP_CALLBACK_URL=https://app.example.com/auth/daptin/callback
daptin-cli oauth app register \
  --name "App Login" \
  --redirect-uri "$APP_CALLBACK_URL" \
  --scope openid \
  --scope profile \
  --scope email \
  --grant authorization_code \
  --grant refresh_token

# Inspect provider-side OAuth apps and rotate a confidential client secret
daptin-cli oauth app list
daptin-cli oauth app describe <client_id_or_reference_id>
daptin-cli oauth app rotate-secret <client_id_or_reference_id>

# Use the returned client_id/client_secret to configure Daptin self-login
export DAPTIN_BASE_URL=https://daptin.example.com
export DAPTIN_SELF_CLIENT_SECRET=...
daptin-cli oauth connect create daptin-login \
  --client-id <client_id> \
  --client-secret-env DAPTIN_SELF_CLIENT_SECRET \
  --auth-url "$DAPTIN_BASE_URL/oauth/authorize" \
  --token-url "$DAPTIN_BASE_URL/oauth/token" \
  --profile-url "$DAPTIN_BASE_URL/oauth/userinfo" \
  --scope openid,profile,email \
  --redirect-uri "$APP_CALLBACK_URL" \
  --profile-email-path email \
  --allow-login \
  --pkce \
  --access-type-offline \
  --update

# Create an OAuth connection without putting the client secret in shell history
export ASANA_CLIENT_SECRET=...
daptin-cli oauth connect create asana.com \
  --client-id "$ASANA_CLIENT_ID" \
  --client-secret-env ASANA_CLIENT_SECRET \
  --auth-url https://app.asana.com/-/oauth_authorize \
  --token-url https://app.asana.com/-/oauth_token \
  --profile-url https://app.asana.com/api/1.0/users/me \
  --scope default

# Start the browser OAuth flow and list token references after callback
daptin-cli oauth login-url asana.com
daptin-cli oauth tokens list --provider asana.com
```

Import large OpenAPI specs from files, URLs, or stdin:

```bash
daptin-cli integration validate-spec --spec-file ./provider.yaml

daptin-cli integration import \
  --provider asana.com \
  --spec-file ./asana_oas.yaml \
  --auth oauth2 \
  --oauth-connect asana.com \
  --update

daptin-cli integration install asana.com
```

Transport extensions can be patched during import for generated facade specs:

```bash
daptin-cli integration import \
  --provider linear.app \
  --spec-file ./linear-openapi.yaml \
  --auth custom_credentials \
  --auth-spec-file ./linear-auth.json \
  --set-operation-transport listIssues=graphql \
  --set-operation-upstream-path listIssues=/graphql \
  --set-graphql-document-file listIssues=./list_issues.graphql \
  --set-graphql-operation-name listIssues=ListIssues \
  --validate
```

For gRPC services without reflection, embed a descriptor set during import:

```bash
daptin-cli integration import \
  --provider grpc.example \
  --spec-file ./grpc-facade.yaml \
  --auth custom_credentials \
  --auth-spec-file ./grpc-auth.json \
  --set-operation-transport Search=grpc \
  --set-grpc-service Search=grpc.testing.SearchService \
  --set-grpc-method Search=Search \
  --grpc-descriptor-set Search=./search.protoset \
  --validate
```

The CLI can also invoke `protoc` with `--grpc-proto Search=./proto/search.proto`
and optional `--grpc-proto-path Search=./proto`; descriptor blobs are embedded
in the imported spec but hidden from normal discovery output.

Discover installed operations through Daptin's scoped integration discovery endpoints:

```bash
daptin-cli integration list
daptin-cli integration operations asana.com
daptin-cli integration operations asana.com --columns operation_id,method,path,transport
daptin-cli integration describe asana.com getWorkspaces
```

Execute provider-scoped operations:

```bash
daptin-cli integration execute asana.com getWorkspaces \
  --oauth-token-id <oauth_token_reference_id> \
  --input-json '{"opt_fields":["name"]}'

daptin-cli integration execute example.com listUsers \
  --credential-id <credential_reference_id> \
  limit=10

daptin-cli integration execute linear.app listIssues \
  --credential-id <credential_reference_id> \
  --input-json '{"first":2,"after":"cursor"}'

daptin-cli integration execute realtime.example wsSearch \
  --credential-id <credential_reference_id> \
  query=tickets

daptin-cli integration execute grpc.example Search \
  --credential-id <credential_reference_id> \
  query=daptin
```

`integration operations` and `integration describe` require Daptin versions with scoped discovery endpoints: `GET /integration/:provider/operations` and `GET /integration/:provider/operations/:operation`.

## Storage

Cloud storage setup and common file operations are available through `storage`.

```bash
# Create a local store
daptin-cli storage add local-files \
  --type local \
  --store-provider local \
  --root-path /tmp/daptin-files

# Create an S3/MinIO-style store and linked credential
daptin-cli storage add minio \
  --type s3 \
  --provider Minio \
  --endpoint http://localhost:9000 \
  --access-key minioadmin \
  --secret-key minioadmin123 \
  --bucket daptin-test \
  --restart

# List and remove stores
daptin-cli storage list
daptin-cli storage remove minio
```

File operations use `store-name:/path` addressing:

```bash
daptin-cli storage mkdir local-files:/photos
daptin-cli storage upload local-files:/photos ./image.jpg
daptin-cli storage upload local-files:/site ./public --recursive
daptin-cli storage mv local-files:/photos/image.jpg local-files:/archive/image.jpg
daptin-cli storage rm local-files:/archive/image.jpg
```

Direct `storage ls` and `storage download` for `cloud_store` paths are intentionally not implemented as direct cloud-store commands because Daptin exposes those flows through site file actions and asset routes, not direct `cloud_store` actions.

## Asset Columns

Use `asset` for `file.*` columns on normal entity rows. These commands use Daptin's `/asset/...` routes.

```bash
daptin-cli asset upload product <product_reference_id> photo ./image.jpg
daptin-cli asset list product <product_reference_id> photo
```

## Output

Table (default) or JSON:

```bash
daptin-cli --output table list world
daptin-cli --output json list world
```

## Filter Syntax

Filters can use `key=value` shorthand or semicolon-separated `<column> <operator> <value>` expressions:

```bash
--filter "name=alice"
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

## E2E Tests

The regular Go test suite stays lightweight:

```bash
go test ./...
```

To run the real integration transport E2E, provide either `DAPTIN_BINARY` or a
local Daptin source checkout via `DAPTIN_SOURCE_DIR` (defaults to `../daptin`):

```bash
DAPTIN_SOURCE_DIR=../daptin ./scripts/integration-transport-e2e.sh
```

This starts a fresh Daptin process, local REST/GraphQL/WebSocket/gRPC upstreams,
and exercises the integration lifecycle using only `daptin-cli` commands.
