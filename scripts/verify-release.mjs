import { readFile, writeFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

export function versionFromTag(tag) {
  if (!/^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)$/.test(tag)) {
    throw new Error(`release tag must match vX.Y.Z, received ${JSON.stringify(tag)}`);
  }
  return tag.slice(1);
}

export function changelogSection(changelog, version) {
  const escaped = version.replaceAll('.', '\\.');
  const start = new RegExp(`^## \\[${escaped}\\](?: - .+)?$`, 'm').exec(changelog);
  if (!start) throw new Error(`CHANGELOG.md has no [${version}] release heading`);
  const bodyStart = start.index + start[0].length;
  const remainder = changelog.slice(bodyStart);
  const next = /^## \[/m.exec(remainder);
  const body = remainder.slice(0, next?.index ?? remainder.length).trim();
  if (!body) throw new Error(`CHANGELOG.md [${version}] section is empty`);
  return body;
}

export async function verifyRelease(tag, outputPath) {
  const version = versionFromTag(tag);
  const changelog = await readFile('CHANGELOG.md', 'utf8');
  const body = changelogSection(changelog, version);
  if (outputPath) await writeFile(outputPath, `${body}\n`, 'utf8');
  return { version, body };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [, , tag, outputPath] = process.argv;
  try {
    const release = await verifyRelease(tag ?? '', outputPath);
    process.stdout.write(`verified ${tag} against CHANGELOG.md (${release.body.length} release-note characters)\n`);
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  }
}
