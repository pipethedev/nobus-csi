# Community package listing (Pulumi Registry)

After the first public GitHub release (`vX.Y.Z`) with plugin binaries and schema, open a PR against [pulumi/registry](https://github.com/pulumi/registry) to list this package.

## Checklist before the PR

1. Public repo at `https://github.com/brimble/pulumi-nobus` (or update `repository` / `pluginDownloadURL` in provider metadata).
2. GitHub Release tagged `v0.1.0` (or later) with multi-arch plugin archives:
   - `pulumi-resource-nobus-v0.1.0-darwin-amd64.tar.gz`
   - `pulumi-resource-nobus-v0.1.0-darwin-arm64.tar.gz`
   - `pulumi-resource-nobus-v0.1.0-linux-amd64.tar.gz`
   - `pulumi-resource-nobus-v0.1.0-linux-arm64.tar.gz`
   - `pulumi-resource-nobus-v0.1.0-windows-amd64.tar.gz`
3. `docs/_index.md` and `docs/installation-configuration.md` present (already in this repo).
4. Schema includes `pluginDownloadURL: github://api.github.com/brimble/pulumi-nobus`.

## Registry PR change

Add an entry to `community-packages/package-list.json` in `pulumi/registry`:

```json
{
  "name": "nobus",
  "repoSlug": "brimble/pulumi-nobus",
  "schemaFile": "schema.json"
}
```

Exact field names may vary — follow the current [publishing guide](https://www.pulumi.com/docs/iac/guides/building-extending/packages/publishing-packages/) and existing community package entries.
