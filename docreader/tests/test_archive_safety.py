import io
import unittest
import zipfile

from docreader.utils.archive import ArchiveLimits, validate_zip_archive


class ArchiveSafetyTest(unittest.TestCase):
    def _archive(self, entries: dict[str, bytes]) -> bytes:
        payload = io.BytesIO()
        with zipfile.ZipFile(payload, "w", zipfile.ZIP_DEFLATED) as archive:
            for name, content in entries.items():
                archive.writestr(name, content)
        return payload.getvalue()

    def test_rejects_total_uncompressed_size_over_budget(self):
        content = self._archive({"a.txt": b"A" * 700, "b.txt": b"B" * 700})
        with self.assertRaisesRegex(ValueError, "uncompressed size"):
            validate_zip_archive(
                content,
                ArchiveLimits(max_members=10, max_member_bytes=1024, max_total_bytes=1024, max_ratio=200),
            )

    def test_rejects_excessive_compression_ratio(self):
        content = self._archive({"bomb.txt": b"A" * 10_000})
        with self.assertRaisesRegex(ValueError, "compression ratio"):
            validate_zip_archive(
                content,
                ArchiveLimits(max_members=10, max_member_bytes=20_000, max_total_bytes=20_000, max_ratio=5),
            )

    def test_rejects_excessive_member_count(self):
        content = self._archive({f"{index}.txt": b"x" for index in range(3)})
        with self.assertRaisesRegex(ValueError, "too many members"):
            validate_zip_archive(
                content,
                ArchiveLimits(max_members=2, max_member_bytes=10, max_total_bytes=10, max_ratio=200),
            )


if __name__ == "__main__":
    unittest.main()
