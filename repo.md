# WCFC-Updater Repository Overview

## Project Summary

**WCFC-Updater** is an automated deployment orchestrator for Wings of Carolina Flying Club (WCFC) applications running on Google Cloud. It automatically triggers builds and deployments when Dependabot updates are available.  WCFC-Updater itself runs as a Google Cloud Run Job, scheduled to run once every six hours.

### Key Workflow
1. Retrieves the current commit from running apps via their `/api/version` endpoints
2. Analyzes git history to detect if auto-updates are needed (checks for Dependabot commits after the latest tag)
3. Validates that no human-written (non-Dependabot) commits exist after the latest tag
4. If conditions are met, triggers the app's build-and-release GitHub Actions workflow via GitHub App authentication

## Directory Structure

```
wcfc-updater/
├── main.go                           # Main entry point and orchestration logic
├── go.mod / go.sum                   # Go module dependencies
├── Dockerfile                        # Container build definition (Fedora-based)
├── Makefile                          # Build, deployment, and development tasks
├── config.yml                        # Configuration listing monitored repositories
├── README.md                         # Project documentation
├── .pre-commit-config.yaml           # Pre-commit hooks for code quality
├── .github/
│   ├── workflows/
│   │   ├── ci_checks.yml             # Pull request and push validation pipeline
│   │   ├── release.yml               # Build and deployment to Google Cloud
│   │   └── dependabot.yml            # Dependabot PR auto-merge and approval
│   └── dependabot.yml                # Dependabot configuration
├── pkg/
│   ├── api_version/version.go        # Fetches commit info from /api/version endpoint
│   └── github_api/github.go          # GitHub API wrapper with JWT/App authentication
└── scripts/
    └── run                           # Container execution script with secret injection
```

## Technologies & Dependencies

### Language & Runtime
- **Go 1.25.1** (CGO disabled for static binary compilation)

### Primary Dependencies
| Package | Purpose |
|---------|---------|
| `github.com/golang-jwt/jwt/v5` | JWT token generation and signing |
| `github.com/google/go-github/v55` | GitHub API client library |
| `gopkg.in/yaml.v3` | YAML configuration parsing |

### Infrastructure
- Docker/Podman containerization
- Fedora Minimal base image
- Google Cloud Artifact Registry for image storage
- Google Cloud Run Jobs for execution

## Key Files

| File | Description |
|------|-------------|
| `main.go` | Main entry point - parses flags, loads config, orchestrates updates |
| `pkg/github_api/github.go` | GitHub App authentication, JWT tokens, commit analysis, workflow dispatch |
| `pkg/api_version/version.go` | Fetches running app version from `/api/version` endpoints |
| `config.yml` | Defines monitored repositories and their hostnames |

## Monitored Repositories

Configured in `config.yml`:

| Repository | Hostname |
|------------|----------|
| wcfc-quiz | quiz.wingsofcarolina.org |
| wcfc-manuals | manuals.wingsofcarolina.org |
| wcfc-groundschool | groundschool.wingsofcarolina.org |
| wcfc-groups | groups-mgmt.wingsofcarolina.org |
| wcfc-updater | (self - no hostname) |

## Build & Deployment

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Compiles Go binary and builds Docker image |
| `make deploy` | Pushes image to GCR and deploys to Cloud Run Jobs |
| `make push` | Pushes image to Google Cloud (requires clean version) |
| `make run` | Executes locally with `--dry-run` flag |
| `make lint` | Runs golangci-lint for code quality |
| `make fmt` | Formats Go code |
| `make check-fmt` | Validates code formatting |
| `make version` | Displays current app version (from git tags) |
| `make clean` | Removes build artifacts |

### Command-Line Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Shows what would be done without making changes |
| `--force` | Bypasses safety checks and forces workflow dispatch |

## CI/CD Pipeline

### GitHub Actions Workflows

#### ci_checks.yml
- **Triggers:** PRs to main, pushes to main
- **Steps:** Checkout → Setup Go → Check formatting → Lint → Build

#### release.yml
- **Triggers:** Manual dispatch or version tag pushes (e.g., `1.2.3`)
- **Steps:** Checkout → Calculate version → Setup Go → Build → GCP Auth → Deploy to Cloud Run Jobs

#### dependabot.yml
- **Purpose:** Automatically approves and merges Dependabot PRs
- **Authentication:** Uses GitHub App token

### Pre-commit Hooks
- Format checking: `make check-fmt`
- Linting: `make lint`

## Environment Variables & Secrets

| Variable | Description |
|----------|-------------|
| `GITHUB_PRIVATE_KEY` | GitHub App private key (base64 or PEM) |
| `GITHUB_APP_ID` | GitHub App identifier |
| `GITHUB_INSTALLATION_ID` | GitHub App installation ID |
| `GOOGLE_CLOUD_JSON` | GCP service account credentials |
| `WCFC_UPDATER_CONFIG` | Optional path to config file |

### Local Development Secrets (Docker/Podman)
- `wcfc-updater-app-id`
- `wcfc-updater-installation-id`
- `wcfc-updater-private-key`

## Execution Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Authentication                                               │
│    - Construct JWT from GitHub App private key                  │
│    - Exchange JWT for installation access token                 │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ 2. Load Configuration                                           │
│    - Parse config.yml for repositories to monitor               │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ 3. Per-Repository Processing                                    │
│    - Fetch latest git tag and commits since tag                 │
│    - Query /api/version for running commit                      │
│    - Validate running version matches tagged version            │
│    - Check if all post-tag commits are from Dependabot          │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ 4. Dispatch (if conditions met)                                 │
│    - Trigger release.yml workflow on target repository          │
│    - Or log dry-run output                                      │
└─────────────────────────────────────────────────────────────────┘
```

