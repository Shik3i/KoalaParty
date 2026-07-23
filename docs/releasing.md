# Releasing

KoalaParty releases are immutable stable SemVer tags matching `vX.Y.Z`.

## Artifacts

Every successful tag release publishes (see `.github/workflows/release.yml`):

- `ghcr.io/shik3i/koalaparty:X.Y.Z`, `X.Y`, `X`, and `latest` for `linux/amd64` and `linux/arm64`;
- an OCI SBOM and provenance metadata for the image;
- a GitHub build-provenance attestation for the image digest;
- a GitHub Release for the tag whose notes are the matching `CHANGELOG.md` section.

The container image is the deployment artifact; there are no separate deployment
bundles or checksum files.

## Checklist

1. Move completed entries from `[Unreleased]` into `## [X.Y.Z] - YYYY-MM-DD`.
2. Run `node scripts/verify-release.mjs vX.Y.Z` (the same tag/changelog check CI runs).
3. Run the verification suite: backend `gofmt`/`go vet`/`go test -race`, `frontend` `npm run check && npm run lint && npm test -- --run && npm run build`, `npm run test:e2e`, and a Docker health smoke test. (`make verify` runs most of this where `make` is available.)
4. Commit and push the release-ready state to `main`; wait for the `CI` workflow to pass. Waiting matters: a tag is immutable, so a red pipeline after tagging burns the version.
5. Create an annotated tag on that exact commit:

   ```sh
   git tag -a vX.Y.Z -m "KoalaParty vX.Y.Z"
   git push origin vX.Y.Z
   ```

6. Monitor the `Release` workflow until all three jobs (`verify`, `image`, `release`) succeed.
7. Verify the auto-created GitHub Release, the image tags, digest, and attestation, and the `/api/version` output from the published image.

Do not move or reuse a published tag. Correct a failed workflow on `main`, then publish a new version tag when the release artifact itself must change.
