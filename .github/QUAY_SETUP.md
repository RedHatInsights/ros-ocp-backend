# Quay.io Container Registry Setup

This document describes the required environment variables and setup needed for the GitHub Actions workflow to build and push container images to Quay.io.

## Required Secrets

The following secrets must be configured at the **organization level** or **repository level** in GitHub:

### Organization Level Secrets (Recommended)

Configure these secrets at the `insights-onprem` organization level to share across all repositories:

1. **`QUAY_USERNAME`** - The username for authenticating with Quay.io
   - Type: Secret
   - Scope: Organization
   - Value: Username of the Quay.io account with push permissions to `quay.io/insights-onprem/`

2. **`QUAY_PASSWORD`** - The password or robot token for authenticating with Quay.io
   - Type: Secret
   - Scope: Organization
   - Value: Password or robot token for the Quay.io account

### Alternative: Repository Level Secrets

If organization-level secrets are not available, configure these at the repository level:

1. **`QUAY_USERNAME`** - Quay.io username
2. **`QUAY_PASSWORD`** - Quay.io password or robot token

## Quay.io Setup Requirements

### 1. Organization Access

Ensure the Quay.io account has the necessary permissions:
- Write access to the `insights-onprem` organization on Quay.io
- Permission to create and push to `quay.io/insights-onprem/ros-ocp-backend` repository

### 2. Robot Account (Recommended)

For better security, create a robot account in Quay.io:

1. Navigate to `quay.io/organization/insights-onprem`
2. Go to "Robot Accounts" section
3. Create a new robot account (e.g., `insights-onprem+github-actions`)
4. Grant "Write" permissions to the robot account
5. Use the robot account credentials as `QUAY_USERNAME` and `QUAY_PASSWORD`

## Workflow Trigger Conditions

The workflow will build and push images when:

1. **Direct push to main branch** - Triggers when relevant files are modified:
   - The workflow file itself (`.github/workflows/build-and-push.yml`)
   - Docker build configuration (`.dockerignore`, `Dockerfile`)
   - Go source code (`cmd/**`, `internal/**`, `rosocp.go`)
   - Go module files (`go.mod`, `go.sum`)
   - Database migrations (`migrations/**`)
   - API specifications (`openapi.json`, `resource_optimization_openshift.json`)
2. **Merged Pull Request** - Creates tags based on the PR and SHA when PR is merged to main
3. **Manual dispatch** - Can be triggered manually from GitHub Actions UI with optional custom tag

## Image Tags

The workflow creates the following tags:

- `latest` - For pushes to main branch
- `main-<sha>` - SHA-based tag for main branch
- `pr-<number>` - For pull request builds (if needed for testing)
- `<custom-tag>` - When manually triggered with a custom tag input

## Manual Workflow Dispatch

To manually trigger the workflow:

1. Navigate to the "Actions" tab in the GitHub repository
2. Select the "Build and Push Container Image" workflow
3. Click "Run workflow" button
4. Optionally specify a custom tag for the image
5. Click "Run workflow" to start the build

This is useful for creating tagged releases or testing specific builds without merging code.

## Final Image Location

The built image will be available at:
```
quay.io/insights-onprem/ros-ocp-backend:latest
quay.io/insights-onprem/ros-ocp-backend:main-<git-sha>
```

This matches the image reference used in the Helm chart at:
```yaml
# deployment/kubernetes/helm/ros-ocp/values.yaml
image:
  repository: quay.io/insights-onprem/ros-ocp-backend
  tag: latest
```
