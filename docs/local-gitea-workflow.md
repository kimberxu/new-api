# Local Gitea Workflow

This repository is maintained with two remotes:

- `origin`: personal Gitea repository, `ssh://git@gitea.228778.xyz:23022/kim/new-api.git`
- `upstream`: original upstream repository, `https://github.com/QuantumNous/new-api.git`

The deployment machine should pull only from `origin`.

## Branches

- `main`: personal main branch on Gitea. Keep it close to upstream plus accepted local commits.
- `deploy`: deployment branch. Production machines should track this branch.
- `local/<feature>`: local customization branches, for example `local/request-debug`.

## Local Development

Start new local customization work from `deploy`:

```bash
git checkout deploy
git pull --ff-only origin deploy
git checkout -b local/request-debug
```

Make changes, run focused checks, then commit:

```bash
git status --short
git add <changed-files>
git commit -m "feat: add request debug logging"
git push -u origin local/request-debug
```

Merge into `deploy` after review and verification:

```bash
git checkout deploy
git pull --ff-only origin deploy
git merge --no-ff local/request-debug
git push origin deploy
```

If the change should also remain visible from `main`, merge `deploy` back:

```bash
git checkout main
git pull --ff-only origin main
git merge --ff-only deploy
git push origin main
```

If `main` cannot fast-forward, inspect the branch history before choosing merge or rebase.

## Deployment Machine

First-time checkout:

```bash
git clone ssh://git@gitea.228778.xyz:23022/kim/new-api.git
cd new-api
git checkout deploy
```

Update an existing deployment:

```bash
cd new-api
git fetch origin
git checkout deploy
git pull --ff-only origin deploy
```

Then run the machine-specific build and restart steps.

Use a read-only SSH deploy key on the deployment machine. Do not put a write-capable personal SSH key on production unless there is a specific reason.

## Syncing Upstream Updates

Fetch upstream into the local machine:

```bash
git fetch upstream
```

Update local `main` with upstream:

```bash
git checkout main
git pull --ff-only origin main
git merge upstream/main
git push origin main
```

Bring `deploy` forward:

```bash
git checkout deploy
git pull --ff-only origin deploy
git merge main
```

Resolve conflicts if needed, then run verification. After checks pass:

```bash
git push origin deploy
```

This merge-based flow is conservative and avoids rewriting remote history. If you later prefer a cleaner local patch stack, use rebase on feature branches, not blindly on a shared deployment branch.

## Reapplying Local Customizations

Keep local customizations small and focused. For request debug logging, the intended shape is:

- no database schema migration in the first version
- no frontend change in the first version
- feature disabled by default
- centralized redaction/truncation/snapshot helper
- snapshots stored under `Other.admin_info.request_debug`

After an upstream update, if conflicts are heavy:

1. create a new branch from updated `main`
2. cherry-pick the local customization commits
3. resolve conflicts in the small set of touched files
4. run focused tests
5. merge the repaired branch into `deploy`

Example:

```bash
git checkout main
git pull --ff-only origin main
git checkout -b local/request-debug-refresh
git cherry-pick <local-request-debug-commit>
```

## Verification Checklist

Before pushing `deploy`, run checks relevant to the touched area. For request debug logging, include:

- focused request debug tests
- relay tests for the changed handler paths
- log formatting tests that confirm `admin_info` is stripped from non-admin log views
- one manual request against a test instance, confirming `request_debug` appears only in admin-visible log details

## Rollback

Tag known-good deployments:

```bash
git checkout deploy
git tag deploy-YYYYMMDD-description
git push origin deploy-YYYYMMDD-description
```

To rollback the deployment machine, checkout the known-good tag or reset the `deploy` branch intentionally after confirming the target commit.
