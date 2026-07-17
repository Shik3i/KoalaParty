import assert from 'node:assert/strict';
import test from 'node:test';
import { changelogSection, versionFromTag } from './verify-release.mjs';

test('accepts stable semantic version tags', () => {
  assert.equal(versionFromTag('v0.1.0'), '0.1.0');
  assert.equal(versionFromTag('v12.34.56'), '12.34.56');
});

test('rejects malformed, prefixed, and prerelease tags', () => {
  for (const tag of ['0.1.0', 'v1', 'vx.x.x', 'v01.2.3', 'v1.2.3-beta.1']) {
    assert.throws(() => versionFromTag(tag), /must match vX\.Y\.Z/);
  }
});

test('extracts only the requested changelog release', () => {
  const changelog = `# Changelog

## [Unreleased]

- Future work.

## [0.2.0] - 2026-08-01

- New release.

## [0.1.0] - 2026-07-17

### Added

- Initial release.
`;
  assert.equal(changelogSection(changelog, '0.1.0'), '### Added\n\n- Initial release.');
  assert.equal(changelogSection(changelog, '0.2.0'), '- New release.');
  assert.throws(() => changelogSection(changelog, '9.9.9'), /no \[9\.9\.9\]/);
});
