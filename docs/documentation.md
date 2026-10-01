# Documentation and publishing

The site uses [Zensical](https://zensical.org/), a custom landing-page layout, and standard documentation pages for the guides and references. Like [Containerlab's documentation setup](https://github.com/srl-labs/containerlab), the source stays beside the code, uv manages the docs environment, and Make orchestrates preview and build.

## Preview locally

From the repository root:

```sh
make docs-serve
```

Open `http://127.0.0.1:8000/`. Zensical rebuilds as you edit. Stop the preview with `Ctrl+C`.

The target installs the pinned uv release from `.github/vars.env` into `.tools/bin` when uv is missing. It then runs the locked `docs` dependency group from `pyproject.toml`; uv can obtain a compatible Python if needed. No system Python packages are modified.

Override the listen address when needed:

```sh
make docs-serve DOCS_ADDR=0.0.0.0:8000
```

## Build and validate

```sh
make docs-build
```

`make docs` is an alias. Both invoke `zensical build --clean --strict` and create `site/`. Strict mode checks internal page links and anchors. The generated site, uv environment, and caches are ignored by Git.

## Cloudflare Workers

The production URL is `https://boxen.containerlab.dev/`, configured in `zensical.toml`. The `boxen` Worker serves the generated `site/` directory using Cloudflare's native static assets. `wrangler.toml` configures the custom domain, trailing-slash page URLs, and the site's `404.html` response. The Worker also enables `workers.dev` URLs for version previews.

Add these GitHub Actions repository secrets:

| Secret | Value |
| --- | --- |
| `CLOUDFLARE_ACCOUNT_ID` | The account containing the Worker and `containerlab.dev` zone |
| `CLOUDFLARE_API_TOKEN` | An API token with permission to deploy Workers and manage the custom domain in that account |

The first production deployment creates the Worker and its custom-domain binding. Cloudflare manages the domain's DNS record and certificate. Subsequent relevant pushes to `main` build and deploy the site automatically. The workflow reads pinned uv, Node, and Wrangler versions from `.github/vars.env`.

For pull requests from this repository, the workflow uploads a Worker version with a `pr-<number>` alias. One bot comment is updated with the immutable commit preview and the alias for the latest PR preview, matching the Clabernetes documentation-preview workflow. Uploading a preview version leaves production running its deployed version. Pull requests from forks build the site without using Cloudflare secrets or publishing comments.

The build job has read-only repository permissions. The deployment job receives `pull-requests: write` for the preview comment. Per-ref concurrency cancels superseded PR runs; production deployments run in order. The bot checks the current PR head before updating a comment so an older build cannot replace links for a newer commit.

For another domain or repository, update `wrangler.toml`, `site_url`, `repo_url`, `repo_name`, and `edit_uri` in `zensical.toml`, and the production URL in the workflow summary. A different default branch also needs corresponding workflow branch filters and the deployment branch condition.

## Publish through Make

After the workflow and docs are present on GitHub, authenticate the GitHub CLI and dispatch the workflow:

```sh
gh auth login
make docs-publish
gh run list --workflow docs.yaml --repo carlmontanari/boxen
```

The target validates the local docs first, then asks GitHub Actions to build **the committed remote `DOCS_REF`**. It does not upload the local `site/` directory or uncommitted edits, and it does not create a commit. The default ref is `main`; only the workflow's main-branch builds deploy to production.

```sh
make docs-publish DOCS_REPO=my-org/boxen DOCS_REF=main
```

The initial checkout work can stay uncommitted for review. Publishing becomes available once the site files and workflow are incorporated into the remote repository by the maintainer.

## Content and styling

Add Markdown pages under `docs/` and link them in the `nav` list in `zensical.toml`. Use relative `.md` links so Zensical resolves them correctly under the project URL. Set page titles and descriptions in front matter where useful.

`docs/index.md` selects `overrides/home.html` and hides its side navigation and table of contents. Other pages retain Zensical's standard navigation, search, table of contents, code-copy buttons, and footer navigation. `docs/stylesheets/boxen.css` sets pure black surfaces in dark mode and pure white surfaces in light mode, with grayscale text, syntax highlighting, and UI accents. The header toggle remembers the selected palette.

The local SVG comes from the Boxen logo linked by the [repository README](https://github.com/carlmontanari/boxen). Its background was removed and its viewBox cropped; light mode inverts the same artwork. The original source URL is recorded in the SVG. The site does not depend on fetching that remote logo at runtime.

## Update dependencies

Change the Zensical version in `pyproject.toml` and regenerate the lockfile with uv:

```sh
uv lock
make docs-build
```

Review the lockfile change and preview both palettes. Keep uv's pinned version in `.github/vars.env`; local installation and the workflow read the same value. Follow [Zensical's customization documentation](https://zensical.org/docs/customization/) when changing theme overrides.
