import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("package-release.py")


class PackageReleaseTest(unittest.TestCase):
    def test_generates_recipes_from_release_checksums(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            checksums = root / "checksums.txt"
            platforms = ("darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64")
            checksums.write_text("".join(
                f"{index:064x}  agrep_0.1.0_{platform}.tar.gz\n"
                for index, platform in enumerate(platforms, 1)
            ))
            output = root / "packaging"
            subprocess.run([sys.executable, SCRIPT, "0.1.0", checksums, output], check=True, capture_output=True)
            formula = (output / "agrep.rb").read_text()
            aur = (output / "PKGBUILD").read_text()
            for index, platform in enumerate(platforms, 1):
                self.assertIn(f"agrep_0.1.0_{platform}.tar.gz", formula)
                self.assertIn(f"{index:064x}", formula)
            self.assertIn("sha256sums_x86_64", aur)
            self.assertIn("sha256sums_aarch64", aur)
            if shutil.which("ruby"):
                subprocess.run(["ruby", "-c", output / "agrep.rb"], check=True, capture_output=True)
            subprocess.run(["bash", "-n", output / "PKGBUILD"], check=True, capture_output=True)

    def test_rejects_missing_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            checksums = root / "checksums.txt"
            checksums.write_text(f"{'1' * 64}  agrep_0.1.0_linux_amd64.tar.gz\n")
            result = subprocess.run(
                [sys.executable, SCRIPT, "0.1.0", checksums, root / "out"],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("missing checksums", result.stderr)


if __name__ == "__main__":
    unittest.main()
