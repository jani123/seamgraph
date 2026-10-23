#!/usr/bin/env python3
"""Install and run the repository's pinned Go quality tools."""
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools/quality"
BIN = ROOT / ".build/quality/bin"


def versions():
    return json.loads((TOOLS / "versions.json").read_text())


def download(url):
    with urllib.request.urlopen(url, timeout=120) as response:
        return response.read()


def install_golangci(version):
    system = platform.system().lower()
    machine = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(platform.machine())
    if system not in ("linux", "darwin") or not machine:
        raise RuntimeError("quality tool installation supports Linux/macOS amd64/arm64")
    release = version.removeprefix("v")
    name = f"golangci-lint-{release}-{system}-{machine}"
    filename = name + ".tar.gz"
    base = f"https://github.com/golangci/golangci-lint/releases/download/{version}/"
    checksums = download(base + f"golangci-lint-{release}-checksums.txt").decode()
    expected = next((line.split()[0] for line in checksums.splitlines() if line.split()[-1] == filename), None)
    data = download(base + filename)
    if expected is None or hashlib.sha256(data).hexdigest() != expected:
        raise RuntimeError("golangci-lint release checksum mismatch")
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        member = archive.getmember(name + "/golangci-lint")
        if not member.isfile():
            raise RuntimeError("golangci-lint archive has no regular executable")
        with archive.extractfile(member) as executable:
            (BIN / "golangci-lint").write_bytes(executable.read())
    (BIN / "golangci-lint").chmod(0o755)


def install():
    BIN.mkdir(parents=True, exist_ok=True)
    stamp = BIN.parent / "installed.json"
    installed = json.loads(stamp.read_text()) if stamp.exists() else {}
    go_version = subprocess.check_output(["go", "version"], text=True).strip()
    for name, spec in versions().items():
        identity = {**spec, "go": go_version}
        if installed.get(name) == identity and (BIN / name).is_file():
            continue
        print(f"Installing {name} {spec['version']}", flush=True)
        if spec.get("distribution") == "github-release":
            install_golangci(spec["version"])
        else:
            subprocess.run(["go", "install", spec["module"] + "@" + spec["version"]],
                           cwd=ROOT, env={**os.environ, "GOBIN": str(BIN), "GOTOOLCHAIN": go_version.split()[2]}, check=True)
        installed[name] = identity
        stamp.write_text(json.dumps(installed, indent=2) + "\n")


REPORTS = ROOT / ".build/quality/reports"
REQUIRED = {"deadcode", "staticcheck", "dupl", "gocyclo", "gocognit", "golangci-lint", "gosec", "govulncheck"}


def package_sources():
    """Use the module's package graph, not recursive filesystem tool discovery."""
    data = subprocess.check_output(["go", "list", "-json", "./..."], cwd=ROOT, text=True)
    decoder = json.JSONDecoder()
    packages, files = [], []
    while data.strip():
        package, end = decoder.raw_decode(data.lstrip())
        data = data.lstrip()[end:]
        directory = Path(package["Dir"])
        packages.append("./" + directory.relative_to(ROOT).as_posix())
        for name in package.get("GoFiles", []) + package.get("CgoFiles", []):
            file = directory / name
            if not re.search(rb"^// Code generated .*DO NOT EDIT\.?\s*$", file.read_bytes(), re.MULTILINE):
                files.append(file.relative_to(ROOT).as_posix())
    if not packages or not files:
        raise RuntimeError("no production Go sources found")
    return sorted(packages), sorted(files)


def complexity_findings(output):
    findings = {}
    for line in output.splitlines():
        if not line.strip():
            continue
        score, _, function, location = line.split()
        file, _, _ = location.rsplit(":", 2)
        identity = file + ":" + function
        if identity in findings:
            raise ValueError("duplicate complexity identity: " + identity)
        findings[identity] = int(score)
    return findings


def complexity_errors(findings, allowances, threshold):
    errors = []
    for identity, score in findings.items():
        limit = allowances.get(identity, {}).get("max", threshold)
        if score > limit:
            errors.append(f"{identity}: complexity {score} exceeds {limit}")
    # An improvement must lower/remove its allowance, making the baseline ratchet
    # down rather than silently preserving room for a later regression.
    for identity, allowance in allowances.items():
        score = findings.get(identity)
        if score is None or score < allowance["max"]:
            errors.append(f"{identity}: lower or remove the stale complexity allowance (now {score or '<= ' + str(threshold)})")
        if not allowance.get("reason") or allowance["max"] <= threshold:
            errors.append(f"{identity}: invalid complexity allowance")
    return errors


def tool_errors(name, process, policy):
    if name in ("gocyclo", "gocognit"):
        if process.returncode not in (0, 1):
            return [f"tool failed with exit {process.returncode}"]
        findings = complexity_findings(process.stdout)
        if process.returncode and not findings:
            return ["tool failed without complexity results"]
        if process.stderr.strip():
            return ["tool reported errors on stderr"]
        return complexity_errors(findings, policy["allowances"][name], policy["thresholds"][name])
    errors = []
    if process.returncode:
        errors.append(f"tool failed with exit {process.returncode}")
    if name in ("deadcode", "dupl") and (process.stdout.strip() or process.stderr.strip()):
        errors.append("tool reported findings (these tools may exit zero despite findings)")
    return errors


def check():
    specs = versions()
    if set(specs) != REQUIRED:
        raise RuntimeError("versions.json must pin all eight required tools")
    missing = [name for name in specs if not (BIN / name).is_file()]
    if missing:
        raise RuntimeError("missing tools: " + ", ".join(missing) + "; run task quality:tools")
    packages, files = package_sources()
    policy = json.loads((TOOLS / "complexity.json").read_text())
    module = subprocess.check_output(["go", "list", "-m"], cwd=ROOT, text=True).strip()
    commands = {
        "deadcode": ["-test", "-filter=^" + re.escape(module) + "/", "./..."],
        "staticcheck": ["./..."],
        "dupl": ["-t", str(policy["thresholds"]["dupl"]), "-plumbing", *files],
        "gocyclo": ["-over", str(policy["thresholds"]["gocyclo"]), *files],
        "gocognit": ["-over", str(policy["thresholds"]["gocognit"]), *files],
        "golangci-lint": ["run", "--config", str(ROOT / ".golangci.yml"), "./..."],
        "gosec": ["-quiet", "-exclude-generated", *packages],
        # Same live Go team database as vuln.go.dev, on its official GCS host.
        "govulncheck": ["-db", os.environ.get("GOVULNDB", "https://storage.googleapis.com/go-vulndb"), "./..."],
    }
    REPORTS.mkdir(parents=True, exist_ok=True)
    results = {}
    for name, arguments in commands.items():
        print(f"Checking {name} {specs[name]['version']}...", flush=True)
        command = [str(BIN / name), *arguments]
        try:
            process = subprocess.run(command, cwd=ROOT, text=True, capture_output=True, timeout=600)
            errors = tool_errors(name, process, policy)
            (REPORTS / (name + ".log")).write_text(process.stdout + process.stderr)
        except (OSError, ValueError, subprocess.TimeoutExpired) as error:
            errors = [str(error)]
            (REPORTS / (name + ".log")).write_text(str(error) + "\n")
        results[name] = {"version": specs[name]["version"], "passed": not errors, "errors": errors}
        suffix = ""
        if name in ("gocyclo", "gocognit"):
            suffix = f" ({len(policy['allowances'][name])} recorded legacy function limits)"
        print(f"{name}: {'FAIL' if errors else 'PASS'}{suffix}", flush=True)
        for error in errors:
            print("  " + error, flush=True)
        if errors:
            print("  Report: " + str(REPORTS / (name + ".log")), flush=True)
    (REPORTS / "summary.json").write_text(json.dumps(results, indent=2) + "\n")
    return all(result["passed"] for result in results.values())


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["install"]:
            install()
        elif sys.argv[1:] == ["check"]:
            sys.exit(0 if check() else 1)
        else:
            raise RuntimeError("usage: python3 scripts/quality.py install|check")
    except (RuntimeError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"quality: {error}", file=sys.stderr)
        sys.exit(1)
