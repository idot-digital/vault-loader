# Vault Loader

A command-line tool that loads secrets from HashiCorp Vault and exports them as environment variables.

## Features

- Automatically detects and uses existing Vault token from:
  - `VAULT_TOKEN` environment variable
  - `VAULT_ID_TOKEN` environment variable (with role-based resolution)
  - AppRole authentication (role ID and secret ID)
  - Current Vault CLI session
- Loads secrets from KV secrets engine
- Supports configuration via:
  - Command line flags
  - Environment variables
  - `.idot.json` config file
- Three modes of operation:
  - Export secrets as bash commands
  - Create a `.env` file
  - Run a command with secrets as environment variables

## Installation

### Quick Install (Linux/macOS)

```bash
curl -s https://vault-loader.idot-digital.com/install.sh | sh
```

### Manual Installation

Visit [vault-loader.idot-digital.com](https://vault-loader.idot-digital.com) to download the latest release for your platform.

### Go Install (Alternative)

```bash
go install github.com/idot-digital/vault-loader@latest
```

### Windows Installation

For Windows users, you can use the Go install command above, or download the Windows binary from the [releases page](https://vault-loader.idot-digital.com).

### Docker

You can use Vault Loader through Docker:

```bash
# Pull the latest version
docker pull idotdigital/vault-loader:latest

# Run vault-loader (example with export command)
docker run --rm -v $PWD:/work -w /work idotdigital/vault-loader:latest export
```

The container supports both amd64 and arm64 architectures. When using Docker, make sure to:

- Mount your working directory to access local files
- Set the working directory to where your configuration files are located
- Pass through any necessary environment variables using `-e` or `--env-file`

## Usage

The tool provides three main commands:

### Export Command

Export secrets as bash commands:

```bash
# Using eval to apply the exports
eval $(vault-loader export)
```

### Env Command

Create a `.env` file with the secrets:

```bash
vault-loader env
```

### Run Command

Run a command with the secrets as environment variables:

```bash
vault-loader run npm start
```

For Docker containers where secrets might be provided via direct environment variables instead of Vault, you can use the `--ignore-if-fail` flag to continue running even when Vault secret loading fails:

```bash
vault-loader run --ignore-if-fail npm start
```

## Configuration

Configuration can be provided in three ways, in order of precedence:

1. Command line flags (highest)
2. Environment variables
3. `.idot.json` config file (lowest)

### Command Line Flags

All commands support the following flags:

- `--path, -p`: Path to the KV secrets (required)
- `--role, -r`: Role to use when resolving ID token (required if VAULT_ID_TOKEN is set)
- `--engine, -e`: Name of the KV secrets engine (default: "kv")
- `--role-id`: Role ID for AppRole authentication
- `--secret-id`: Secret ID for AppRole authentication
- `--unquoted`: Export secrets without quotes (default: false)

The `run` command also supports:

- `--ignore-if-fail`: Continue running command even if secret loading fails

### Environment Variables

The following environment variables can be used to configure the tool:

- `VAULT_LOADER_PATH`: Path to the KV secrets (required)
- `VAULT_LOADER_ROLE`: Role to use when resolving ID token
- `VAULT_LOADER_ENGINE`: Name of the KV secrets engine (default: "kv")

### Config File

You can provide configuration in a `.idot.json` file:

```json
{
  "secrets": {
    "path": "secret/my-app",
    "role": "my-role",
    "engine": "kv"
  }
}
```

## Vault Environment Variables

The following environment variables are used for Vault authentication:

- `VAULT_TOKEN`: Direct access token for Vault
- `VAULT_ID_TOKEN`: ID token for role-based authentication
- `VAULT_ROLE_ID`: Role ID for AppRole authentication
- `VAULT_SECRET_ID`: Secret ID for AppRole authentication
- `VAULT_ADDR`: Vault server address (optional, defaults to https://127.0.0.1:8200)

## Examples

```bash
# Using command line flags
vault-loader export --path secret/my-app

# Using environment variables
export VAULT_LOADER_PATH=secret/my-app
vault-loader export

# Using config file
vault-loader export

# Creating .env file
vault-loader env

# Running a command with secrets
vault-loader run npm start

# Using VAULT_TOKEN
export VAULT_TOKEN=your-token
vault-loader export

# Using VAULT_ID_TOKEN with role
export VAULT_ID_TOKEN=your-id-token
vault-loader export --role my-role

# Using AppRole authentication
export VAULT_ROLE_ID=your-role-id
export VAULT_SECRET_ID=your-secret-id
vault-loader export

# Using AppRole authentication with command line flags
vault-loader export --role-id your-role-id --secret-id your-secret-id

# Using a different KV engine
vault-loader export --engine secret

# Running a command even if Vault secrets fail to load
vault-loader run --ignore-if-fail npm start
```
