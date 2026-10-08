---
name: 2026-10-08-changing-a-standard-doc-description-does-not-change-the-cata
description: Caution record for a solved false case or recurring risk.
---

# Changing a standard doc description does not change the catalog of repos that already have frontmatter

- Date: 2026-10-08
- Kind: `caution`
- Source: issueops-docs #555
- Summary: The project-doc catalog prefers each document's frontmatter description and falls back to meta.go only when it is empty, so editing docMetaDescriptions alone leaves the hook output of existing repos unchanged until project bootstrap --sync rewrites their frontmatter (#555).
- Context: In #555 the meta.go descriptions were rewritten to the '<what>; read <when>.' form. The rebuilt hook still printed the old descriptions for this repo, because buildProjectDocCatalogEntry reads the frontmatter first and every standard doc here already had one. ADR.md and OPERATIONS.md also carried repo-specific descriptions that differed from the canonical text.
- Resolution: When a change edits docMetaDescriptions, sync the frontmatter of this repo's standard docs in the same change (EnsureMetaFrontmatter gives the canonical block) and check the rendered catalog with a real hook run, not only the meta.go unit test. State in the PR that other repos pick up the new text only on their next project bootstrap --sync.
