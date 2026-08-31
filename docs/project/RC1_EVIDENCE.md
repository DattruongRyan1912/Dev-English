# RC1 evidence matrix

Last updated: 2026-08-23

This file is the audit index for the `20260823-0035_rc1-hardening-next-plan` directive. A local build, a valid Compose file or a green unit test is not treated as proof of an external production deployment.

| Gate | Current status | Evidence | Remaining condition |
| --- | --- | --- | --- |
| P0.1 secure web auth | PASS locally | Production bundle scan, anonymous rejection, Chrome login/reload/logout and session expiry tests | Verify `Secure` cookie on the real HTTPS origin |
| P0.2 DeepSeek secret lifecycle | PASS locally | Settings set/replace/remove/test, encrypted PostgreSQL storage, metadata-only GET and safe logs/tests | Re-run once on the production secret store |
| P0.3 provider probes | PASS | DeepSeek model, Groq STT, Azure Pronunciation and Azure TTS capability probes with safe errors and live smoke | Keep live credentials out of repository and logs |
| P0.4 migration/recovery | PASS locally | Versioned migrations, transactional migration recording, guarded restore, backup/restore rehearsal and atomic writing outcome | Repeat backup/restore on production storage |
| P0.5 HTTPS/CORS | PARTIAL | Strict HTTPS origin validation, credentialed cookie CORS tests and local production-origin smoke | Real DNS, certificate and public-origin verification |
| P1 browser voice | PASS for real Chrome | Microphone permission → recording → Groq STT → PostgreSQL SpeakingSession → Azure assessment/prosody → Azure TTS playback | Additional browser/device is optional practical follow-up |
| P2 PostgreSQL learning loop | PASS locally | Browser/API/PostgreSQL evidence for diagnostic, writing, mistakes/skills, review, roleplay, Copilot, work context and public GitHub import | Private GitHub provisioning remains deferred by directive |
| P3 automated regression | PASS | Go tests/race/vet, Flutter analyze/test/build, browser smoke, API smoke and disposable PostgreSQL repository integration test | Keep the live-provider smoke opt-in |
| P4 release discipline | PARTIAL | Review-fix is present only in the current uncommitted worktree; local and remote branch HEAD remain `d39019a`; CI run 32632728390 covers `d39019a` only; rebuilt-backend Chrome smoke and local migration regression passed | Commit/push the review-fix, obtain CI on the exact new SHA and one independent approval, then merge and record the exact merge SHA before marking PASS |
| P5 real production | PARTIAL | Production Docker/Caddy artifacts, runbook and local production-mode `/healthz`/`/readyz`/auth/CORS smoke | Operator must provide domain/DNS/host/production DB/secrets and execute deployment |
| P6 dogfood/sign-off | PENDING | Cannot start before P5 | Use the deployed app as the real learner and record critical issues/sign-off |

## Release decision

RC1 is not marked ready or complete until P5 external deployment evidence and P6 dogfood/sign-off exist. The repository is currently deployable in principle, but no production host, domain, DNS, certificate or production secret set is present in the repository or GitHub configuration.
