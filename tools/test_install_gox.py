import hashlib
import io
import tarfile
import unittest

from install_gox import unpack


def archive(entries):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode="w:gz") as tar:
        for name, data in entries.items():
            member = tarfile.TarInfo(name)
            member.size = len(data)
            tar.addfile(member, io.BytesIO(data))
    return stream.getvalue()


class GoXArchiveTests(unittest.TestCase):
    def test_reads_only_executable_and_license(self):
        data = archive({"./gox": b"binary", "./LICENSE": b"notice", "../../outside": b"ignored"})
        self.assertEqual(unpack(data, hashlib.sha256(data).hexdigest()), {"gox": b"binary", "LICENSE": b"notice"})

    def test_rejects_corrupt_archive_before_reading_it(self):
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            unpack(b"not a tar archive", "0" * 64)

    def test_rejects_archive_without_license(self):
        data = archive({"gox": b"binary"})
        with self.assertRaisesRegex(ValueError, "license"):
            unpack(data, hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    unittest.main()
