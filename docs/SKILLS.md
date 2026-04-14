# Skills

This repo publishes agent skills from the root `skills/` directory so they can be discovered by the `skills` CLI without any repo-specific conventions.

## Layout

Put each skill in its own folder:

```text
skills/
└── <skill-name>/
    ├── SKILL.md
    ├── references/
    └── scripts/
```

Required:

- `SKILL.md` with YAML frontmatter containing `name` and `description`

Optional:

- `references/` for compact capability notes
- `scripts/` for runnable helpers or demos that the skill can point to

## Repo convention

- Treat `README.md` as the public source of truth for user-facing capability claims.
- Keep skill-specific notes small and link back to the README instead of duplicating large feature matrices.
- Use relative links that work from inside `skills/<skill-name>/`.
- Do not add speculative syntax or unsupported commands to a skill.

## Local validation

List discoverable skills:

```bash
npx skills add . --list
```

Test-install one skill from the local checkout:

```bash
npx skills add . --skill qql-skill --copy
```

Install from GitHub after publishing:

```bash
npx skills add srimon12/qql-go --skill qql-skill
```

## Adding a new skill

1. Create `skills/<skill-name>/SKILL.md`.
2. Add any tightly related helper files under that same folder.
3. Reference the new skill from `README.md` if it is part of the public repo surface.
4. Run `npx skills add . --list` to confirm the skill is discoverable.
