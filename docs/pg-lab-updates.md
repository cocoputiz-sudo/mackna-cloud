# MackNa Cloud · PostgreSQL Lab and agent updates (Node 01)

## State and boundaries
- PostgreSQL Manager Phase A uses only the disposable `mackna_lab` database as `mackna_reader`.
- The MackNa Agent stays on `127.0.0.1:8090`; use an SSH tunnel to open `/postgres/lab`.
- The PostgreSQL lab is exposed on **host loopback only** at `127.0.0.1:55432`.
- Never connect this feature to the IMORTAIS PostgreSQL volume/container or touch `mackna-btv-*`.
- Read-only HTTP access uses a scoped MackNa token. Do not paste a token into screenshots.
- The UI reads schemas, tables, rows (20/page), columns, indexes and relationships through bounded API endpoints. It deliberately offers no arbitrary SQL or write UI.

## Release artifact provenance
The PR CI builds the ARM64 agent off-device, fails if the compiled binary does not embed the external UI JavaScript and restrictive CSP, stamps `build` with the source commit and publishes:
- unique GitHub Actions artifact name (run number/attempt);
- `dist/SHA256SUMS` with the actual agent binary digest;
- `dist/BUILD_INFO.txt` with exact source commit.

Never assume the constant product version (`0.3.0`) is proof of build identity. After installation, verify `GET /api/v1/health` includes the expected `build` SHA.

## GitHub Releases updater (opt-in, not yet installed)
Workflow `.github/workflows/release-agent.yml` runs **only on `main` by manual `workflow_dispatch`** with a semver version like `0.3.1`. It tests, cross-compiles, verifies the embedded web assets and publishes `agent-v0.3.1` with `mackna-agent-linux-arm64`, `SHA256SUMS` and `BUILD_INFO.txt`.

After the PR has been reviewed, merged and a stable agent release has been explicitly published:
```bash
# root on BTV11; first install the updater script from a trusted reviewed commit
bash /root/update-agent-release.sh --check
bash /root/update-agent-release.sh --apply
# OPTIONAL: begin 12-hour automatic checks for published stable releases
bash /root/update-agent-release.sh --enable-auto
# Stop scheduled updates later:
sudo /usr/local/sbin/mackna-agent-updater --disable-auto
```

The updater contacts **only the fixed GitHub repository**, checks release metadata, binary SHA256, ELF ARM64 architecture, external UI fingerprint and compiled source commit. On apply it updates **only** `/usr/local/bin/mackna-agent`, restarts **only** `mackna-agent.service`, validates the server-reported commit, and restores the previous agent on failure.

Security notes:
- The timer is **not** enabled by the GitHub PR, CI or installer. It requires explicit operator action.
- A public GitHub release can be fetched without a personal access token on the node.
- Checksums published alongside artifacts protect against accidental corruption and mismatch; they **do not** substitute for GitHub repository/release access controls or cryptographic artifact signing.
- Running updates automatically can still briefly interrupt **the MackNa control-plane interface**; it must not touch IMORTAIS app/database, Docker daemon or Docker Gate.
- Do not enable auto-update until a successful **manual** release update and rollback exercise has been validated on the BTV11.

## Acceptance
1. Navigate the PostgreSQL Manager via SSH tunnel and observe real metrics, tables and sample fixture rows.
2. Confirm unauthorized routes return HTTP 401 and lab-only routes return 404 with lab disabled.
3. Confirm the UI loads external `/postgres/lab.js` under CSP, and buttons operate without inline JavaScript.
4. Verify package build SHA and checksum before agent-only installation.
5. Validate no unrelated containers or volumes were changed.
