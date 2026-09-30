import hashlib
import os
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path
import unittest

SCRIPT = Path(__file__).with_name("install.sh")


class InstallScriptTest(unittest.TestCase):
    def test_syntax_sh_and_bash(self):
        if shutil.which("sh"):
            subprocess.run(["sh", "-n", SCRIPT], check=True, capture_output=True)
        if shutil.which("bash"):
            subprocess.run(["bash", "-n", SCRIPT], check=True, capture_output=True)

    def test_script_is_executable(self):
        self.assertTrue(SCRIPT.stat().st_mode & 0o111 != 0, "install.sh should be executable")

    def test_end_to_end_mock_install(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            work = Path(tmpdir)
            bindir = work / "bin"
            bindir.mkdir()
            mock_server_dir = work / "server"
            mock_server_dir.mkdir()

            uname_s = subprocess.check_output(["uname", "-s"]).decode().strip().lower()
            uname_m = subprocess.check_output(["uname", "-m"]).decode().strip()
            arch = "amd64" if uname_m in ("x86_64", "amd64") else ("arm64" if uname_m in ("aarch64", "arm64") else uname_m)
            tarball_name = f"agrep_0.1.0_{uname_s}_{arch}.tar.gz"

            dummy_bin = work / "agrep"
            dummy_bin.write_text("#!/bin/sh\necho agrep version 0.1.0\n")
            dummy_bin.chmod(0o755)

            tarball_path = mock_server_dir / tarball_name
            with tarfile.open(tarball_path, "w:gz") as tar:
                tar.add(dummy_bin, arcname="agrep")

            tar_bytes = tarball_path.read_bytes()
            sha256 = hashlib.sha256(tar_bytes).hexdigest()
            checksums_path = mock_server_dir / "checksums.txt"
            checksums_path.write_text(f"{sha256}  {tarball_name}\n")

            mock_downloader = work / "downloader.sh"
            mock_downloader.write_text(f"""#!/bin/sh
case "$1" in
  *checksums.txt) cp "{checksums_path}" "$2" ;;
  *{tarball_name}) cp "{tarball_path}" "$2" ;;
  *) echo "Unknown mock URL $1" >&2 ; exit 1 ;;
esac
""")
            mock_downloader.chmod(0o755)

            env = dict(os.environ)
            env["BINDIR"] = str(bindir)
            env["VERSION"] = "v0.1.0"
            env["CUSTOM_DOWNLOADER"] = str(mock_downloader)

            proc = subprocess.run([str(SCRIPT)], env=env, capture_output=True, text=True)
            self.assertEqual(proc.returncode, 0, f"STDOUT: {proc.stdout}\nSTDERR: {proc.stderr}")
            installed_agrep = bindir / "agrep"
            self.assertTrue(installed_agrep.exists(), "agrep binary should be installed")
            self.assertTrue(installed_agrep.stat().st_mode & 0o111 != 0, "agrep binary should be executable")


if __name__ == "__main__":
    unittest.main()
