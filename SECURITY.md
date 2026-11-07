# Security Policy

## Supported versions

Security fixes are applied to the latest release and the `main` branch. Older
builds are not maintained.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting:

**https://github.com/srikar-up/openleet/security/advisories/new**

Do not open a public issue, discussion, or pull request for a vulnerability.

Include:

- The affected version or commit.
- Operating system and terminal.
- A minimal reproduction, if possible.
- The impact you believe it has.
- Whether the issue involves local data, provider keys, OAuth, sync, or the
  runner.

You can expect an initial response within a few days. A fix and disclosure
timeline will be agreed in the advisory.

## Scope

OpenLeet handles sensitive local state. Reports are especially useful for:

- The encrypted provider-key vault (`internal/secrets`).
- Encrypted local identity and session storage (`internal/identity`).
- Firebase authentication, refresh-token handling, and Firestore rules
  (`internal/network`, `firestore.rules`).
- Command execution and process-tree cleanup in the host runner
  (`internal/runner`), including injection through problem fixtures or file
  paths.
- Terminal escape-sequence injection through rendered problem content.
- Path traversal in storage, import/export, or backup tools.

## Out of scope

- Vulnerabilities in third-party terminals, shells, Python, Go, or AI providers.
- Issues that require an already-compromised local account.
- Denial of service from deliberately huge problem files.
- Reports that only show a dependency version is outdated without a concrete
  exploit path.

## Safe harbour

Good-faith research that follows this policy will not be treated as a violation
of the project's terms. Do not access other users' data, degrade the service, or
publish details before a fix is available.
