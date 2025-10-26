# ☁️ cf-delete-worker

A beautiful CLI tool for safely deleting Cloudflare Workers and their associated resources.

## Features

- 🎨 **Beautiful TUI** - Powered by Bubble Tea and Lipgloss with Cloudflare's brand colors
- 🔒 **Safe Deletion** - Prevents accidental deletion of shared resources
- 🔍 **Dependency Analysis** - Scans all workers to find shared resources
- 🌈 **Interactive Mode** - Step-by-step confirmations with clear explanations
- 🏃 **Dry Run Mode** - Preview what will be deleted without making changes
- 🔑 **Secure Credentials** - API keys stored securely in your config directory

## Installation

### From Source

```bash
git clone https://github.com/cloudflare/cf-delete-worker.git
cd cf-delete-worker
go build -o cf-delete-worker
sudo mv cf-delete-worker /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/cloudflare/cf-delete-worker@latest
```

## Prerequisites

You'll need a Cloudflare API Token with the following permissions:

- **Workers Scripts**: Edit
- **Workers KV Storage**: Edit
- **Workers R2 Storage**: Edit
- **Workers D1**: Edit
- **Account Settings**: Read

Create a token at: https://dash.cloudflare.com/profile/api-tokens

## Quick Start

1. **Delete a worker (interactive mode)**:
   ```bash
   cf-delete-worker my-worker
   ```

2. **Preview deletion without making changes**:
   ```bash
   cf-delete-worker --dry-run my-worker
   ```

3. **Delete only exclusive resources (preserve shared ones)**:
   ```bash
   cf-delete-worker --exclusive-only my-worker
   ```

## Usage

```bash
cf-delete-worker [flags] <worker-name>
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--account-id <id>` | | Specify Cloudflare account ID |
| `--dry-run` | `-d` | Show deletion plan without executing |
| `--force` | `-f` | Skip confirmation prompts (dangerous) |
| `--exclusive-only` | | Only delete resources not shared with other workers |
| `--yes` | `-y` | Answer yes to all prompts |
| `--verbose` | `-v` | Verbose logging |
| `--quiet` | `-q` | Minimal output |
| `--json` | | Output results in JSON format |
| `--update-key` | | Update stored API key |
| `--help` | `-h` | Show help message |
| `--version` | | Show version information |

### Examples

**Basic deletion with interactive confirmations**:
```bash
cf-delete-worker my-api-worker
```

**Dry run to see what would be deleted**:
```bash
cf-delete-worker --dry-run my-api-worker
```

**Delete only exclusive resources, preserve shared ones**:
```bash
cf-delete-worker --exclusive-only my-api-worker
```

**Force deletion without prompts (use with caution!)**:
```bash
cf-delete-worker --force --yes my-api-worker
```

**Use with specific account**:
```bash
cf-delete-worker --account-id abc123def456 my-worker
```

**Update stored API token**:
```bash
cf-delete-worker --update-key
```

## How It Works

1. **Authentication**: On first run, you'll be prompted for your Cloudflare API token. It's stored securely in `~/.config/cf-delete-worker/credentials`.

2. **Worker Discovery**: The tool fetches details about the specified worker, including all its bindings (KV namespaces, R2 buckets, D1 databases, etc.).

3. **Dependency Analysis**: All workers in your account are scanned to identify which resources are shared vs. exclusive to the target worker.

4. **Interactive Confirmation**: You're shown a detailed deletion plan and asked to confirm before any changes are made.

5. **Safe Deletion**: Resources are deleted in the correct order, with clear progress indication and error handling.

## Supported Resource Types

- ✅ KV Namespaces
- ✅ R2 Buckets
- ✅ D1 Databases
- ✅ Durable Objects (bindings)
- ✅ Service Bindings
- ✅ Queue Bindings
- ✅ Environment Variables
- ✅ Secrets

## Configuration

### Environment Variables

- `CLOUDFLARE_API_TOKEN`: API token (for CI/CD, overrides stored token)

### Config File

API credentials are stored in:
- Linux/macOS: `~/.config/cf-delete-worker/credentials`
- Windows: `%APPDATA%\cf-delete-worker\credentials`

## Safety Features

- **Multi-step confirmation** for destructive operations
- **Shared resource warnings** with detailed usage information
- **Dry-run mode** to preview changes
- **Exclusive-only mode** to preserve shared resources
- **Color-coded risk indicators**
- **Clear error messages** with recovery suggestions

## Development

### Building from Source

```bash
git clone https://github.com/cloudflare/cf-delete-worker.git
cd cf-delete-worker
go mod download
go build -o cf-delete-worker
```

### Running Tests

```bash
go test ./...
```

### Project Structure

```
cf-delete-worker/
├── cmd/              # CLI commands
├── internal/
│   ├── api/          # Cloudflare API client
│   ├── auth/         # Authentication & credentials
│   ├── analyzer/     # Dependency analysis
│   ├── deleter/      # Deletion orchestration
│   └── ui/           # Bubble Tea TUI components
│       ├── models/   # UI state models
│       ├── views/    # View renderers
│       └── styles/   # Lipgloss styles
├── pkg/
│   └── types/        # Shared types
└── main.go           # Entry point
```

## Troubleshooting

### "No accounts found"
Make sure your API token has the correct permissions and is associated with a Cloudflare account.

### "Multiple accounts found"
Specify the account ID with `--account-id <id>`.

### "Failed to delete worker"
Check that the worker exists and your API token has Workers Scripts: Edit permission.

### "Permission denied" errors
Your API token may not have sufficient permissions. Create a new token with the required scopes.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss)
- Uses the official [Cloudflare Go SDK](https://github.com/cloudflare/cloudflare-go)

## Support

- GitHub Issues: https://github.com/cloudflare/cf-delete-worker/issues
- Cloudflare Community: https://community.cloudflare.com/

---

**⚠️ Warning**: This tool performs destructive operations. Always use `--dry-run` first to preview changes!
