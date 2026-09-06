"""Make ``gust_sdk`` importable without installing the package.

Lets `python -m pytest sdk/python` work from the repository root, which is how
CI and the demo scripts invoke it.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
