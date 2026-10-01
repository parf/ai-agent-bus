"""Build metadata follows the repository version; installed wheels use theirs."""
from importlib.metadata import PackageNotFoundError, version
from pathlib import Path

_source = Path(__file__).resolve().parents[3] / "internal/version/VERSION"
if _source.exists():
    VERSION = _source.read_text().strip()
else:
    try:
        VERSION = version("agent-bus-client")
    except PackageNotFoundError as exc:
        raise RuntimeError("install agent-bus-client before importing it") from exc
