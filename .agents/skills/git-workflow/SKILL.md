---
name: git-workflow
description: Manage task branches, focused commits, pull requests, milestone tags, and source-version handoffs to CI/CD. Use for requested Git changes or planning delivery workflows; do not initialize, publish, merge, tag, or deploy merely because code was edited.
---

# Git delivery workflow

Preserve existing work and make each delivered version traceable. User authorization persists within its stated scope; a commit request alone does not authorize pushing, merging, tagging, or deploying. Follow already-authorized actions without repeatedly asking permission.

## Inspect and branch

- Confirm the repository root, current branch, working tree, staged diff, remotes, and relevant history before mutations. If Git is not initialized, initialize it only when requested.
- Identify existing user changes and staged files. Do not overwrite, discard, or silently include them in the task. Avoid destructive resets, cleans, history rewrites, and force pushes without explicit authorization.
- For an agreed implementation slice, use a short descriptive feature or fix branch when the workflow calls for it. Preserve the existing branch if the task is to continue its work. Base new work on the agreed branch or revision; do not assume local main matches the remote.

## Implement, verify, and commit

- Keep changes scoped to the agreed slice. Run checks appropriate to the behavior and the repository's documented commands. Documentation-only edits need documentation checks, not unrelated benchmarks.
- Inspect the final diff and whitespace checks. Check ignore rules before initial staging: local configuration/secrets, editor files, generated binaries, profiles, and runtime data should not enter the commit; retain safe examples and reproducibility evidence intentionally.
- Stage explicit task paths or hunks. Review the staged diff before committing, especially when unrelated files are already staged.
- Make each commit coherent and describe its purpose. Prefixes such as feat:, fix:, docs:, test:, and chore: are useful conventions; do not split a change into artificial commits to demonstrate Git usage.
- Record the resulting commit SHA and working-tree state. Report any uncommitted or pre-existing changes left behind.

## Pull request and merge

- Push and open a PR when authorized. Describe the problem, final behavior, verification, and relevant limitations for a reviewer without conversation context.
- For small completed slices, squash merge is the default recommendation; preserve meaningful commit history when the agreed repository workflow needs it. Use the repository's actual merge policy.
- Require the configured checks and review before merging. Check the latest PR revision and mergeability; do not claim branch protection or CI exists until it has been configured and verified.
- Treat merge as a separate delivery action unless included in the user's authorization. Use the merged commit as the source reference for milestone tags and build artifacts.

## Versions and milestone tags

- Use annotated tags for agreed milestones, with the exact version and target commit confirmed by the task or repository policy. Inspect existing local and remote tags first where publication is involved.
- Use vMAJOR.MINOR.PATCH for application releases: incompatible public-contract changes affect major versions after 1.0, compatible additions minor versions, and fixes patch versions. Before 1.0, document compatibility expectations. Use prerelease suffixes for versions that are not final.
- A guidance/template milestone may be tagged, but label it as such; it is not a runnable application release. Do not invent a release version or tag every commit.
- Keep published tags fixed. Corrections require a new version rather than silently moving or deleting the existing tag. Creating a tag, pushing it, and publishing release notes are distinct actions; respect the authorized scope.
- Release notes state the milestone, changes, verification, and any migration or compatibility implications.

## Contract with a separate CI/CD project

- The application repository owns source, API contracts, and reproducible build/check commands. A separate CI/CD project can own pipeline execution, deployment configuration, credentials, and approvals.
- Recommended event contract: PR updates run checks; main merges build artifacts; release tags validate and promote the artifact for the exact source commit; deployment follows the environment's approval policy. This is a design until integrations are implemented.
- Identify artifacts by source commit SHA, plus a readable version when tagged, and retain the artifact digest and relevant build/pipeline revision. Promote the same artifact through environments rather than rebuilding per environment.
- A tag or branch name does not prove successful checks or deployment. Verify pipeline outcomes and source/artifact mapping before reporting release readiness.
- Keep credentials outside source control. Configure CI triggers, release publication, and deployment only within the task's authorization; Git operations may activate existing automation, so inspect known triggers before publishing.
