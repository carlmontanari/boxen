module.exports = async function ({ github, context, core, kind }) {
  const { owner, repo } = context.repo;
  const pull = context.payload.pull_request;
  const { data: current } = await github.rest.pulls.get({ owner, repo, pull_number: pull.number });
  if (current.state !== 'open' || current.head.sha !== pull.head.sha) {
    core.info('Skipping comment for a closed pull request or superseded commit.');
    return;
  }

  let body;
  const marker = `<!-- boxen-${kind} -->`;
  const commit = pull.head.sha.slice(0, 7);
  if (kind === 'docs') {
    const output = (process.env.WRANGLER_OUTPUT || '').replace(/\x1b\[[0-9;]*m/g, '');
    const previewUrl = (label) => {
      const match = output.match(new RegExp(`${label}:\\s+(https://[^\\s]+)`));
      if (!match) throw new Error(`Missing ${label}; enable Worker preview URLs.`);
      const url = new URL(match[1]);
      if (url.protocol !== 'https:' || !url.hostname.endsWith('.workers.dev')) {
        throw new Error(`Invalid ${label}: ${url}`);
      }
      return url.href;
    };
    body = [
      marker, '## Documentation preview', '',
      '| | URL |', '|---|---|',
      `| Commit \`${commit}\` | ${previewUrl('Version Preview URL')} |`,
      `| PR #${pull.number} | ${previewUrl('Version Preview Alias URL')} |`,
    ].join('\n');
  } else if (kind === 'binaries') {
    const artifacts = await github.paginate(github.rest.actions.listWorkflowRunArtifacts, {
      owner, repo, run_id: context.runId, per_page: 100,
    });
    const rows = [];
    for (const os of ['linux', 'darwin']) {
      for (const arch of ['amd64', 'arm64']) {
        const name = `boxen-${os}-${arch}`;
        const artifact = artifacts.find((item) => item.name === name && !item.expired);
        if (!artifact) throw new Error(`Artifact not found: ${name}`);
        const url = `https://github.com/${owner}/${repo}/actions/runs/${context.runId}/artifacts/${artifact.id}`;
        rows.push(`| ${os}/${arch} | [${name}](${url}) |`);
      }
    }
    body = [
      marker, '<details>', `<summary>Download Boxen CLI binaries for commit ${commit}</summary>`, '',
      '| Platform | Download |', '|---|---|', ...rows, '',
      'Artifacts require GitHub sign-in and are retained for 14 days.', '',
      'For Linux AMD64 (change the artifact name for another platform):', '',
      '```sh',
      `gh run download ${context.runId} --repo ${owner}/${repo} --name boxen-linux-amd64 --dir ./boxen-ci`,
      'chmod +x ./boxen-ci/boxen', './boxen-ci/boxen --version', '```', '', '</details>',
    ].join('\n');
  } else {
    throw new Error(`Unknown comment kind: ${kind}`);
  }

  await core.summary.addRaw(body).write();
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner, repo, issue_number: pull.number, per_page: 100,
  });
  const existing = comments.find((comment) =>
    comment.user?.login === 'github-actions[bot]' && comment.body?.includes(marker));
  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number: pull.number, body });
  }
};
