"""Resource limits for ZIP-based document formats."""

from __future__ import annotations

import io
import zipfile
from dataclasses import dataclass


@dataclass(frozen=True)
class ArchiveLimits:
    max_members: int = 2_000
    max_member_bytes: int = 100 * 1024 * 1024
    max_total_bytes: int = 200 * 1024 * 1024
    max_ratio: float = 200.0


DEFAULT_ARCHIVE_LIMITS = ArchiveLimits()


def validate_zip_archive(
    content: bytes, limits: ArchiveLimits = DEFAULT_ARCHIVE_LIMITS
) -> None:
    """Reject a ZIP whose declared expansion can exceed safe parser budgets."""
    with zipfile.ZipFile(io.BytesIO(content)) as archive:
        members = archive.infolist()
        if len(members) > limits.max_members:
            raise ValueError(
                f"archive has too many members ({len(members)} > {limits.max_members})"
            )

        total = 0
        for member in members:
            if member.is_dir():
                continue
            if member.file_size > limits.max_member_bytes:
                raise ValueError(
                    f"archive member exceeds uncompressed size limit: {member.filename}"
                )
            total += member.file_size
            if total > limits.max_total_bytes:
                raise ValueError("archive exceeds total uncompressed size limit")
            ratio = member.file_size / max(member.compress_size, 1)
            if ratio > limits.max_ratio:
                raise ValueError(
                    f"archive member exceeds compression ratio limit: {member.filename}"
                )
