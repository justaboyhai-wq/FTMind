#!/usr/bin/env python3
"""Interactively update only the Tair password in the local .env file."""

from getpass import getpass
from pathlib import Path


path = Path(__file__).with_name(".env")
if not path.is_file():
    raise SystemExit(".env not found")

password = getpass("Tair password (input hidden): ")
if not password:
    raise SystemExit("Password cannot be empty")

lines = path.read_text(encoding="utf-8").splitlines()
updated = False
output = []
for line in lines:
    if line.startswith("REDIS_PASSWORD="):
        output.append("REDIS_PASSWORD=" + password)
        updated = True
    else:
        output.append(line)

if not updated:
    raise SystemExit("REDIS_PASSWORD entry not found")

path.write_text("\n".join(output) + "\n", encoding="utf-8")
path.chmod(0o600)
print("Tair password updated")
