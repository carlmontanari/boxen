const assert = require('node:assert/strict');
const { test } = require('node:test');
const comment = require('./pr-comment.cjs');

test('previews and binary links update one bot comment and skip outdated pull requests', async () => {
  const sha = '0123456789abcdef';
  const writes = [];
  let current = { state: 'open', head: { sha } };
  let comments = [];
  const artifacts = ['linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64'].map((platform, i) => ({
    name: `boxen-${platform}`, id: i + 1, expired: false,
  }));
  const github = {
    rest: {
      pulls: { get: async () => ({ data: current }) },
      actions: { listWorkflowRunArtifacts: 'artifacts' },
      issues: {
        listComments: 'comments',
        createComment: async (value) => writes.push({ operation: 'create', ...value }),
        updateComment: async (value) => writes.push({ operation: 'update', ...value }),
      },
    },
    paginate: async (method) => method === 'artifacts' ? artifacts : comments,
  };
  const context = { repo: { owner: 'owner', repo: 'boxen' }, runId: 42,
    payload: { pull_request: { number: 7, head: { sha } } } };
  const core = { info() {}, summary: { addRaw() { return this; }, async write() {} } };
  const args = { github, context, core };
  const originalOutput = process.env.WRANGLER_OUTPUT;
  try {
    process.env.WRANGLER_OUTPUT = '\x1b[32mVersion Preview URL: https://abc-boxen.example.workers.dev\x1b[0m\n' +
      'Version Preview Alias URL: https://pr-7-boxen.example.workers.dev';
    await comment({ ...args, kind: 'docs' });
    assert.equal(writes[0].operation, 'create');
    assert.match(writes[0].body, /Commit `0123456`/);
    assert.match(writes[0].body, /https:\/\/abc-boxen\.example\.workers\.dev\//);
    assert.match(writes[0].body, /https:\/\/pr-7-boxen\.example\.workers\.dev\//);
    comments = [
      { id: 1, user: { login: 'someone' }, body: '<!-- boxen-docs -->' },
      { id: 2, user: { login: 'github-actions[bot]' }, body: writes[0].body },
    ];
    await comment({ ...args, kind: 'docs' });
    assert.equal(writes[1].operation, 'update');
    assert.equal(writes[1].comment_id, 2);
    await comment({ ...args, kind: 'binaries' });
    for (const artifact of artifacts) {
      assert.ok(writes[2].body.includes(`[${artifact.name}]`));
      assert.ok(writes[2].body.includes(`/actions/runs/42/artifacts/${artifact.id}`));
    }
    assert.match(writes[2].body, /gh run download 42 --repo owner\/boxen/);
    artifacts[0].expired = true;
    await assert.rejects(comment({ ...args, kind: 'binaries' }), /Artifact not found/);
    process.env.WRANGLER_OUTPUT = '';
    await assert.rejects(comment({ ...args, kind: 'docs' }), /Missing Version Preview URL/);
    process.env.WRANGLER_OUTPUT = 'Version Preview URL: https://example.com/';
    await assert.rejects(comment({ ...args, kind: 'docs' }), /Invalid Version Preview URL/);
    for (const value of [{ state: 'closed', head: { sha } }, { state: 'open', head: { sha: 'new' } }]) {
      current = value;
      await comment({ ...args, kind: 'docs' });
      await comment({ ...args, kind: 'binaries' });
    }
    assert.equal(writes.length, 3);
  } finally {
    if (originalOutput === undefined) delete process.env.WRANGLER_OUTPUT;
    else process.env.WRANGLER_OUTPUT = originalOutput;
  }
});
