---
name: git-workflow
description: Git conventions and healthy workflow for this repository, including thematic English commits, reviewing changes before committing, and correct push usage. Use when committing, amending, staging, pushing, or creating pull requests.
origin: ECC
---

# Git Workflow

Healthy git habits that keep history readable, atomic, and safe. These complement
the validation rules in the project `AGENTS.md` (English only, lefthook hooks).

## When to Activate

- Creating, amending, or reverting commits
- Staging or unstaging changes
- Pushing, pulling, or setting up remote tracking
- Creating pull requests
- Reviewing the diff before finalizing a change

## Core Rules

### 1. Commit Only When Asked

Never commit unless the user explicitly asks. When asked, commit only the
intended files. Inspect the state first:

```bash
git status
git diff          # unstaged changes
git diff --cached # staged changes
git log --oneline -10
```

Stage only what belongs to the change. Never commit secrets, build artifacts, or
unrelated files.

### 2. Thematic, English Commits

Group work into focused commits (one logical change each) rather than one large
"everything" commit or microscopic noise commits. Use messages in English that
match the repo style, with a conventional `type(scope): summary` prefix:

```text
feat(k8smonitor): implement Phase 1 Kubernetes TLS monitor
chore(dev): add disposable Kubernetes dev environment
docs(k8s): document Phase 1 monitor and dev environment
fix(cli): sort certificates by days-left before output
test(checker): add table-driven cases for leaf parsing
```

Keep the subject concise (imperative mood), and use a body only when the change
needs explanation.

### 3. Inspect Before Finalizing

Before every commit, review `git diff` for correctness, accidental changes, and
stray files (e.g. editor swap files, `coverage.out`, build output). If a commit
fails or a hook rejects it, fix the issue and create a **new commit** — do not
amend the failing commit unless the user requested it.

### 4. Hooks Run on Commit/Push

The lefthook hooks (see `AGENTS.md`) run on commit and push. Ensure they pass:

```bash
lefthook run pre-commit
lefthook run pre-push
```

If a hook reports a problem, address it rather than bypassing hooks
(e.g. `--no-verify` should be a last resort and only when the user agrees).

### 5. Push Deliberately

Only push when asked. Prefer `git push` on the current branch with tracking set
up. Do not force-push or rewrite published history without explicit approval.

```bash
git push -u origin branch-name   # first time, set tracking
git push                          # subsequent times
```

## Anti-Patterns

- Committing with `git add .` blindly and pulling in unrelated changes
- Splitting one logical change across many "wip"/"tmp" commits
- Portuguese or vague messages (`atualiza coisa`, `fix`, `misc`)
- Amending a rejected/failed commit repeatedly to make checks pass
- Pushing to `main` without review or before the request

**Remember**: history is documentation for future readers. Make each commit
small, focused, in English, and passing the project's hooks.
