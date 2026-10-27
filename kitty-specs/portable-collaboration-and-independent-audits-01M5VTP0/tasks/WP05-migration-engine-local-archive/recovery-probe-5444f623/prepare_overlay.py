"""Materialize the frozen probe overlay for a chosen product checkout. Does not run tests or edit that checkout."""
import argparse
import json
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("checkout", help="Existing nichthub checkout to test")
parser.add_argument("output", help="New or existing temporary directory for overlay material")
args = parser.parse_args()
checkout = Path(args.checkout).resolve(strict=True)
output = Path(args.output).resolve()
if not (checkout / "go.mod").is_file() or not (checkout / "migration_engine_test.go").is_file():
    parser.error("checkout must contain go.mod and migration_engine_test.go")
output.mkdir(parents=True, exist_ok=True)
evidence = Path(__file__).resolve().parent
parts = ["baseline-migration_engine_test.go.txt", "missing-progress-probe.go.txt", "destination-change-probe.go.txt"]
material = "\n".join((evidence / name).read_text() for name in parts)
replacement = output / "migration_engine_test.go"
replacement.write_text(material)
overlay = output / "overlay.json"
overlay.write_text(json.dumps({"Replace": {str(checkout / "migration_engine_test.go"): str(replacement)}}, indent=2) + "\n")
print(overlay)
