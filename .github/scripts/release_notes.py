#!/usr/bin/env python3
"""Print the CHANGELOG.md section of a release, and check it matches the code.

Usage:
  release_notes.py v1.2.3            print the notes of `## [v1.2.3] — YYYY-MM-DD`
  release_notes.py v1.2.3 --check    also fail unless the SDK version in gogemini/client.go is "1.2.3"

Used by the Release workflow on tag push; runs locally the same way. Standard library only.
"""

import re
import sys

CHANGELOG = "CHANGELOG.md"
VERSION_FILE = "gogemini/client.go"
TAG_RE = re.compile(r"^v\d+\.\d+\.\d+$")


def section(tag):
    with open(CHANGELOG, encoding="utf-8") as fh:
        lines = fh.read().splitlines()
    heading = re.compile(rf"^## \[{re.escape(tag)}\] — \d{{4}}-\d{{2}}-\d{{2}}\s*$")
    out, inside = [], False
    for line in lines:
        if line.startswith("## "):
            if inside:
                break
            inside = bool(heading.match(line))
            continue
        if inside and not re.match(r"^\[[^\]]+\]: ", line):
            out.append(line)
    if not inside and not out:
        return None
    return "\n".join(out).strip()


def code_version():
    with open(VERSION_FILE, encoding="utf-8") as fh:
        m = re.search(r'\bversion\s*=\s*"([^"]+)"', fh.read())
    return m.group(1) if m else None


def main():
    args = sys.argv[1:]
    if not args or not TAG_RE.match(args[0]):
        sys.exit("usage: release_notes.py vX.Y.Z [--check]")
    tag, check = args[0], "--check" in args[1:]

    notes = section(tag)
    if not notes:
        sys.exit(f"error: no dated `## [{tag}] — YYYY-MM-DD` section with content in {CHANGELOG}")
    if check:
        v = code_version()
        if v != tag[1:]:
            sys.exit(f"error: {VERSION_FILE} has version = {v!r}, tag is {tag}")
    print(notes)
    print(f"\n**Full changelog**: https://github.com/Allan-Nava/go-gemini/blob/{tag}/{CHANGELOG}")


if __name__ == "__main__":
    main()
