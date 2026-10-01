function previewUrl(value, label) {
  if (!value) throw new Error(`Missing ${label}; enable Worker preview URLs.`);
  const url = new URL(value);
  if (url.protocol !== 'https:' || !url.hostname.endsWith('.workers.dev')) {
    throw new Error(`Invalid ${label}: ${url}`);
  }
  return url.href;
}

function parsePreviewOutput(output) {
  const clean = (output || '').replace(/\x1b\[[0-9;]*m/g, '');
  const read = (label) => previewUrl(
    clean.match(new RegExp(`${label}:\\s+(https://[^\\s]+)`))?.[1], label);
  return { previewUrl: read('Version Preview URL'), aliasUrl: read('Version Preview Alias URL') };
}

module.exports = async function ({ github, context, core }) {
  const { owner, repo } = context.repo;
  const pull = context.payload.pull_request;
  const isCurrent = async () => {
    const { data } = await github.rest.pulls.get({ owner, repo, pull_number: pull.number });
    return data.state === 'open' && data.head.sha === pull.head.sha;
  };
  if (!await isCurrent()) {
    core.info('Skipping comment for a closed pull request or superseded commit.');
    return;
  }

  const marker = '<!-- boxen-builds -->';
  const runUrl = `https://github.com/${owner}/${repo}/actions/runs/${context.runId}`;
  const lines = [marker, `## Boxen builds for commit \`${pull.head.sha.slice(0, 7)}\``, '',
    '### Documentation preview', ''];
  if (process.env.DOCS_STATUS === 'success') {
    lines.push('| | URL |', '|---|---|',
      `| Commit preview | ${previewUrl(process.env.DOCS_PREVIEW_URL, 'Version Preview URL')} |`,
      `| PR #${pull.number} | ${previewUrl(process.env.DOCS_ALIAS_URL, 'Version Preview Alias URL')} |`);
  } else {
    lines.push(`Documentation preview: **${process.env.DOCS_STATUS || 'unavailable'}**. [View CI run](${runUrl}).`);
  }

  lines.push('', '<details>', '<summary>Download Boxen CLI binaries</summary>', '',
    'Downloads require GitHub sign-in and are retained for 14 days. Commands use `gh auth token`, `curl`, and `unzip`.', '');
  if (process.env.BINARY_STATUS === 'success') {
    const artifacts = await github.paginate(github.rest.actions.listWorkflowRunArtifacts, {
      owner, repo, run_id: context.runId, per_page: 100,
    });
    for (const os of ['linux', 'darwin']) {
      for (const arch of ['amd64', 'arm64']) {
        const name = `boxen-${os}-${arch}`;
        const artifact = artifacts.find((item) => item.name === name && !item.expired);
        if (!artifact) throw new Error(`Artifact not found: ${name}`);
        const pageUrl = `${runUrl}/artifacts/${artifact.id}`;
        const apiUrl = `https://api.github.com/repos/${owner}/${repo}/actions/artifacts/${artifact.id}/zip`;
        lines.push(`### ${os === 'darwin' ? 'macOS' : 'Linux'} ${arch.toUpperCase()}`, '',
          `[${name}](${pageUrl})`, '', '```sh',
          'curl --fail --silent --show-error --location \\',
          '  -H "Authorization: Bearer $(gh auth token)" \\',
          `  "${apiUrl}" \\`,
          `  -o ${name}.zip && unzip -p ${name}.zip boxen > ${name}.tmp && \\`,
          `  mv ${name}.tmp ${name} && rm ${name}.zip && chmod +x ${name}`,
          '```', '');
      }
    }
  } else {
    lines.push(`CLI build: **${process.env.BINARY_STATUS || 'unavailable'}**. [View CI run](${runUrl}).`, '');
  }
  lines.push('</details>');
  const body = lines.join('\n');
  await core.summary.addRaw(body).write();

  const comments = await github.paginate(github.rest.issues.listComments, {
    owner, repo, issue_number: pull.number, per_page: 100,
  });
  const markers = [marker, '<!-- boxen-docs -->', '<!-- boxen-binaries -->'];
  const matching = comments.filter((comment) => comment.user?.login === 'github-actions[bot]' &&
    markers.some((value) => comment.body?.includes(value)));
  const existing = matching.find((comment) => comment.body.includes(marker)) || matching[0];
  // Recheck after collecting build data so a superseded run cannot replace newer links.
  if (!await isCurrent()) return;
  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
    for (const duplicate of matching.filter((comment) => comment.id !== existing.id)) {
      await github.rest.issues.deleteComment({ owner, repo, comment_id: duplicate.id });
    }
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number: pull.number, body });
  }
};

module.exports.parsePreviewOutput = parsePreviewOutput;
