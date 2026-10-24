# Semantic Version & Docker Image Publishing Workflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a GitHub Actions workflow that automatically tags releases using PaulHatch/semantic-version and pushes backend/frontend Docker images to Docker Hub.

**Architecture:** A single workflow triggered on push to main branch. It uses PaulHatch/semantic-version to calculate the next semantic version from conventional commits, builds both backend and frontend Docker images with multi-platform support, and pushes them to Docker Hub with version tags and `latest`.

**Tech Stack:** GitHub Actions, PaulHatch/semantic-version@v5, docker/build-push-action@v5, docker/login-action@v3, Docker Hub

---

## Task 1: Create the GitHub Workflows Directory

**Files:**
- Create: `.github/workflows/` (directory)

**Step 1: Create the workflows directory**

Run:
```bash
mkdir -p .github/workflows
```

**Step 2: Verify directory exists**

Run: `ls -la .github/`
Expected: `workflows` directory exists

**Step 3: Commit**

```bash
git add .github/
git commit -m "chore: create GitHub workflows directory"
```

---

## Task 2: Create the CI/CD Workflow File

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the release workflow**

Create file `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    branches:
      - main

permissions:
  contents: write

jobs:
  version:
    name: Calculate Version
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.semver.outputs.version }}
      version_tag: ${{ steps.semver.outputs.version_tag }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Calculate Semantic Version
        id: semver
        uses: paulhatch/semantic-version@v5.4.0
        with:
          tag_prefix: "v"
          major_pattern: "(BREAKING CHANGE:|!:)"
          minor_pattern: "feat:"
          version_format: "${major}.${minor}.${patch}"
          bump_each_commit: false
          search_commit_body: true

      - name: Display Version
        run: |
          echo "Version: ${{ steps.semver.outputs.version }}"
          echo "Version Tag: ${{ steps.semver.outputs.version_tag }}"
          echo "Version Type: ${{ steps.semver.outputs.version_type }}"

  build-backend:
    name: Build & Push Backend
    runs-on: ubuntu-latest
    needs: version
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and Push Backend
        uses: docker/build-push-action@v5
        with:
          context: ./backend
          file: ./backend/Dockerfile
          push: true
          platforms: linux/amd64,linux/arm64
          tags: |
            seacodebase/urlshortener-backend:${{ needs.version.outputs.version }}
            seacodebase/urlshortener-backend:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max

  build-frontend:
    name: Build & Push Frontend
    runs-on: ubuntu-latest
    needs: version
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and Push Frontend
        uses: docker/build-push-action@v5
        with:
          context: ./frontend
          file: ./frontend/Dockerfile
          push: true
          platforms: linux/amd64,linux/arm64
          tags: |
            seacodebase/urlshortener-frontend:${{ needs.version.outputs.version }}
            seacodebase/urlshortener-frontend:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max

  tag-release:
    name: Create Git Tag
    runs-on: ubuntu-latest
    needs: [version, build-backend, build-frontend]
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Create and Push Tag
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          TAG="v${{ needs.version.outputs.version }}"
          if ! git rev-parse "$TAG" >/dev/null 2>&1; then
            git tag -a "$TAG" -m "Release $TAG"
            git push origin "$TAG"
            echo "Created and pushed tag: $TAG"
          else
            echo "Tag $TAG already exists, skipping"
          fi
```

**Step 2: Verify YAML syntax**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo "YAML is valid"
```
Expected: "YAML is valid"

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add release workflow with semantic versioning and Docker push"
```

---

## Task 3: Document Required GitHub Secrets

**Files:**
- Modify: `README.md` or create documentation note

**Step 1: Document the required secrets**

The workflow requires two secrets to be configured in GitHub repository settings:

| Secret Name | Description |
|-------------|-------------|
| `DOCKERHUB_USERNAME` | Docker Hub username (e.g., `seacodebase`) |
| `DOCKERHUB_TOKEN` | Docker Hub access token (NOT password) |

To configure:
1. Go to GitHub repo → Settings → Secrets and variables → Actions
2. Click "New repository secret"
3. Add `DOCKERHUB_USERNAME` with value `seacodebase`
4. Add `DOCKERHUB_TOKEN` with a Docker Hub access token

To create a Docker Hub access token:
1. Log in to Docker Hub
2. Go to Account Settings → Security → Access Tokens
3. Click "New Access Token"
4. Give it a name like "github-actions-urlshortener"
5. Set permissions to "Read & Write"
6. Copy the token and save it as the GitHub secret

**Step 2: Inform user about secrets**

No commit needed for this step - this is informational for the user.

---

## Task 4: Test Workflow Locally (Optional Verification)

**Step 1: Verify workflow file exists and is valid**

Run:
```bash
ls -la .github/workflows/release.yml
```
Expected: File exists

**Step 2: Check git status**

Run: `git status`
Expected: Clean working tree or only untracked files

**Step 3: Push to trigger workflow**

Run:
```bash
git push origin main
```

**Step 4: Monitor workflow**

Check workflow status at:
`https://github.com/SeaCodeBase/urlshortener/actions`

Expected: Workflow runs, but may fail on Docker push if secrets not configured yet.

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create workflows directory | `.github/workflows/` |
| 2 | Create release workflow | `.github/workflows/release.yml` |
| 3 | Document required secrets | Documentation |
| 4 | Test workflow | Push to main |

## Workflow Behavior

1. **Trigger**: On every push to `main` branch
2. **Version Calculation**: Uses conventional commits to determine version bump
   - `feat:` → minor version bump (e.g., 1.0.0 → 1.1.0)
   - `fix:`, `chore:`, etc. → patch version bump (e.g., 1.0.0 → 1.0.1)
   - `BREAKING CHANGE:` or `!:` → major version bump (e.g., 1.0.0 → 2.0.0)
3. **Docker Images**: Builds and pushes to Docker Hub
   - `seacodebase/urlshortener-backend:1.2.3` and `:latest`
   - `seacodebase/urlshortener-frontend:1.2.3` and `:latest`
4. **Git Tag**: Creates `v1.2.3` tag after successful builds

## Required GitHub Secrets

- `DOCKERHUB_USERNAME`: `seacodebase`
- `DOCKERHUB_TOKEN`: Docker Hub access token with read/write permissions
