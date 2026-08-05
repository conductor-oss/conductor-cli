# Python deploy fixture

Minimal project used by `test/e2e/deploy.bats` to exercise `conductor deploy`
discovery and deployment against a live server.

`conductor deploy` shells out to `python -m agentspan.cli.discover`, so the
`agentspan` package must be importable. The suite creates a `.venv` here on first
run; `conductor deploy` auto-detects `<project>/.venv/bin/python`.

Setup performed by the suite (or run manually):

```bash
python3 -m venv .venv
./.venv/bin/pip install -r requirements.txt
```

Note `--path` discovery skips `__init__.py`, so the agents live in `agents.py`.
