# Source-only backup

Commit and verify the repository, then run from its root:

```sh
python3 scripts/source-backup.py
# Windows:
py -3 scripts/source-backup.py
```

The script archives the committed Git revision into a new ZIP beside the
checkout. It refuses pending changes, an existing output file, output inside
the checkout, and tracked secrets/build/runtime artifacts. Dependencies are
reproducible from `go.sum` and `frontend/pnpm-lock.yaml`. Fonts, their licences,
embedded migrations, deployment definitions, tests and documentation remain
in the archive. `.env.example` is a template without real credentials.

To choose a location, use `--output /protected/location/leadomi-source.zip`.
Extract into a fresh directory and follow [project structure](PROJECT_STRUCTURE.md)
to install dependencies and build. The archive omits Git history; GitHub holds
the pushed history. Record the full revision printed by the script.

After verification, remove local compiled outputs and dependency caches with
`python3 scripts/clean-generated.py` (Windows: `py -3 scripts/clean-generated.py`).
This keeps runtime data, credentials and local temporary work intact.

Source archives contain no live database, recordings, PBX credentials or
Coolify environment. Recovering the hosted system also requires the protected
[runtime backups](BACKUP.md) and deployment configuration described there.
