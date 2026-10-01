const assert = require('node:assert/strict');
const { test } = require('node:test');
const comment = require('./pr-comment.cjs');

test('one comment survives migration, new commits, failures, and stale runs', async () => {
  const sha = '0123456789abcdef';
  let current = { state: 'open', head: { sha } };
  let comments = [
    { id: 1, user: { login: 'someone' }, body: '<!-- boxen-docs -->' },
    { id: 2, user: { login: 'github-actions[bot]' }, body: '<!-- boxen-docs -->\nOld preview' },
    { id: 3, user: { login: 'github-actions[bot]' }, body: '<!-- boxen-binaries -->\nOld binaries' },
  ];
  const artifacts = ['linux-amd64', 'linux-arm64', 'darwin-amd64', 'darwin-arm64'].map((platform, i) => ({
    name: `boxen-${platform}`, id: i + 1, expired: false,
  }));
  const writes = [];
  let supersedeDuringRead = false;
  const github = {
    rest: {
      pulls: { get: async () => ({ data: current }) },
      actions: { listWorkflowRunArtifacts: 'artifacts' },
      issues: {
        listComments: 'comments',
        createComment: async (value) => {
          writes.push({ operation: 'create', ...value });
          comments.push({ id: 100, user: { login: 'github-actions[bot]' }, body: value.body });
        },
        updateComment: async (value) => {
          writes.push({ operation: 'update', ...value });
          comments.find((item) => item.id === value.comment_id).body = value.body;
        },
        deleteComment: async (value) => {
          writes.push({ operation: 'delete', ...value });
          comments = comments.filter((item) => item.id !== value.comment_id);
        },
      },
    },
    paginate: async (method) => {
      if (method === 'artifacts') return artifacts;
      if (supersedeDuringRead) current = { state: 'open', head: { sha: 'newer' } };
      return comments;
    },
  };
  const context = { repo: { owner: 'owner', repo: 'boxen' }, runId: 42,
    payload: { pull_request: { number: 7, head: { sha } } } };
  const core = { info() {}, summary: { addRaw() { return this; }, async write() {} } };
  const args = { github, context, core };
  const values = { DOCS_STATUS: 'success', BINARY_STATUS: 'success',
    DOCS_PREVIEW_URL: 'https://abc-boxen.example.workers.dev',
    DOCS_ALIAS_URL: 'https://pr-7-boxen.example.workers.dev' };
  const original = Object.fromEntries(Object.keys(values).map((key) => [key, process.env[key]]));
  Object.assign(process.env, values);
  try {
    await comment(args);
    assert.deepEqual(writes.map((item) => [item.operation, item.comment_id]), [['update', 2], ['delete', 3]]);
    assert.equal(comments.length, 2); // Keep the user's comment.
    const body = comments.find((item) => item.id === 2).body;
    assert.match(body, /Boxen builds for commit `0123456`/);
    assert.match(body, /abc-boxen\.example\.workers\.dev/);
    assert.match(body, /pr-7-boxen\.example\.workers\.dev/);
    assert.equal(body.match(/```sh/g).length, 4);
    for (const artifact of artifacts) {
      assert.ok(body.includes(`/actions/runs/42/artifacts/${artifact.id}`));
      assert.ok(body.includes(`/actions/artifacts/${artifact.id}/zip`));
      assert.ok(body.includes(`-o ${artifact.name}.zip && unzip -p ${artifact.name}.zip boxen`));
      assert.ok(body.includes(`chmod +x ${artifact.name}`));
    }
    context.payload.pull_request.head.sha = 'abcdef0123456789';
    current = { state: 'open', head: { sha: context.payload.pull_request.head.sha } };
    context.runId = 43;
    process.env.DOCS_PREVIEW_URL = 'https://new-boxen.example.workers.dev';
    await comment(args);
    assert.equal(writes.at(-1).comment_id, 2);
    assert.equal(comments.length, 2);
    assert.match(writes.at(-1).body, /commit `abcdef0`/);
    assert.match(writes.at(-1).body, /new-boxen/);
    assert.match(writes.at(-1).body, /actions\/runs\/43\/artifacts/);
    assert.ok(!writes.some((item) => item.operation === 'create'));

    artifacts[0].expired = true;
    await assert.rejects(comment(args), /Artifact not found/);
    artifacts[0].expired = false;
    process.env.DOCS_PREVIEW_URL = 'https://example.com/';
    await assert.rejects(comment(args), /Invalid Version Preview URL/);
    Object.assign(process.env, values);
    process.env.DOCS_STATUS = 'failure';
    await comment(args);
    assert.match(writes.at(-1).body, /Documentation preview: \*\*failure\*\*/);
    assert.equal(writes.at(-1).body.match(/```sh/g).length, 4);
    process.env.DOCS_STATUS = 'success';
    process.env.BINARY_STATUS = 'failure';
    await comment(args);
    assert.match(writes.at(-1).body, /CLI build: \*\*failure\*\*/);
    assert.ok(!writes.at(-1).body.includes('```sh'));

    comments = [];
    Object.assign(process.env, values);
    await comment(args);
    await comment(args);
    assert.equal(comments.length, 1);
    assert.equal(writes.filter((item) => item.operation === 'create').length, 1);
    assert.equal(writes.at(-1).comment_id, 100);
    const count = writes.length;
    supersedeDuringRead = true;
    await comment(args);
    for (const value of [{ state: 'closed', head: current.head }, { state: 'open', head: { sha: 'older' } }]) {
      current = value;
      await comment(args);
    }
    assert.equal(writes.length, count);
  } finally {
    for (const [key, value] of Object.entries(original)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
});

test('parse Wrangler preview URLs, including ANSI formatting', () => {
  const urls = comment.parsePreviewOutput('\x1b[32mVersion Preview URL: https://abc-boxen.example.workers.dev\x1b[0m\n' +
    'Version Preview Alias URL: https://pr-7-boxen.example.workers.dev');
  assert.deepEqual(urls, { previewUrl: 'https://abc-boxen.example.workers.dev/',
    aliasUrl: 'https://pr-7-boxen.example.workers.dev/' });
  assert.throws(() => comment.parsePreviewOutput(''), /Missing Version Preview URL/);
  assert.throws(() => comment.parsePreviewOutput('Version Preview URL: https://example.com/'), /Invalid Version Preview URL/);
  assert.throws(() => comment.parsePreviewOutput('Version Preview URL: https://abc.example.workers.dev'), /Missing Version Preview Alias URL/);
});
