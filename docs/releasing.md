# Releasing

KoalaParty releases are immutable stable SemVer tags matching `vX.Y.Z`.

## Artifacts

Every successful tag release publishes:

- `ghcr.io/shik3i/koalaparty:X.Y.Z`, `X.Y`, `X`, and `latest` for `linux/amd64` and `linux/arm64`;
- an OCI SBOM and provenance metadata;
- a GitHub build-provenance attestation for the image digest;
- `koalaparty-vX.Y.Z-deploy.tar.gz` and `.zip` deployment bundles;
- `SHA256SUMS` for both bundles;
- a GitHub Release whose notes come from the matching `CHANGELOG.md` section.

## Checklist

1. Move completed entries from `[Unreleased]` into `## [X.Y.Z] - YYYY-MM-DD`.
2. Run `node scripts/verify-release.mjs vX.Y.Z`.
3. Run `make verify`, `npm run test:e2e` in `frontend`, the Go race detector, and a Docker health smoke test.
4. Commit and push the release-ready state to `main`; wait for CI to pass.
5. Create an annotated tag on that exact commit:

   ```sh
   git tag -a vX.Y.Z -m "KoalaParty vX.Y.Z"
   git push origin vX.Y.Z
   ```

6. Monitor the `Release` workflow until all jobs succeed.
7. Verify the GitHub Release, deployment archives, `SHA256SUMS`, image tags, digest, attestation, and `/api/version` output from the published image.

Do not move or reuse a published tag. Correct a failed workflow on `main`, then publish a new version tag when the release artifact itself must change.
