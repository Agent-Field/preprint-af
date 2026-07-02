"""Entrypoint: launch the preprint-af AgentField node.

Adds the local `src/` tree to the import path so `python main.py` works without an
editable install; a `pip install .` (see pyproject.toml) works the same way.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "src"))

from preprint_af.app import build_app  # noqa: E402

app = build_app()

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.getenv("PORT", "8001")), auto_port=False)
