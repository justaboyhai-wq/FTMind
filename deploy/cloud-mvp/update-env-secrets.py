#!/usr/bin/env python3
"""Interactively update the external credential fields in a local .env file.

The script never prints entered secrets. It is intentionally kept out of the
container image and is used only by an administrator on the ECS host.
"""

from getpass import getpass
from pathlib import Path


ENV_FILE = Path(__file__).with_name(".env")
REQUIRED_KEYS = ("REDIS_PASSWORD", "OSS_ACCESS_KEY", "OSS_SECRET_KEY")


def main() -> None:
    if not ENV_FILE.is_file():
        raise SystemExit(f"missing {ENV_FILE}; create it from .env.example first")

    redis_password = getpass("Tair password (input hidden): ")
    oss_access_key = input("OSS AccessKey ID: ").strip()
    oss_secret_key = getpass("OSS AccessKey Secret (input hidden): ")
    if not all((redis_password, oss_access_key, oss_secret_key)):
        raise SystemExit("all three values are required; .env was not changed")

    lines = ENV_FILE.read_text().splitlines()
    updates = {
        "REDIS_PASSWORD": redis_password,
        "OSS_ACCESS_KEY": oss_access_key,
        "OSS_SECRET_KEY": oss_secret_key,
    }
    seen = {line.split("=", 1)[0] for line in lines if "=" in line}
    missing = set(REQUIRED_KEYS) - seen
    if missing:
        raise SystemExit("missing keys in .env: " + ", ".join(sorted(missing)))

    rewritten = []
    for line in lines:
        key = line.split("=", 1)[0]
        rewritten.append(f"{key}={updates[key]}" if key in updates else line)

    ENV_FILE.write_text("\n".join(rewritten) + "\n")
    ENV_FILE.chmod(0o600)
    print(".env credentials updated")


if __name__ == "__main__":
    main()
