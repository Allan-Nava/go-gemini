#!/usr/bin/env python3
"""Sync docs/milestone.md to GitHub milestones and issues.

The markdown file is the source of truth:

  ## vX.Y.Z — Title (optional note)     -> milestone "vX.Y.Z — Title"
  **Obiettivo:** text                   -> milestone description
  - [ ] <!-- id:some-id --> **Title**: …  -> open issue, label `milestone-sync`
  - [x] <!-- id:some-id --> …           -> closed issue (completed)
  item removed from the file            -> issue closed as not planned

Issues are matched by the hidden marker `<!-- milestone-sync:id=… -->` in their body, so the
sync is idempotent; duplicates for one id are closed, keeping the oldest open issue. An issue closed on GitHub while still `[ ]` in the file is left closed and
reported: tick it in the file instead of reopening it.

Usage:
  milestone_sync.py --check     parse and validate only, no network (used on pull requests)
  milestone_sync.py --dry-run   read GitHub and print the plan, change nothing
  milestone_sync.py             apply

Token: GITHUB_TOKEN or GH_TOKEN, else `gh auth token`. Repo: GITHUB_REPOSITORY or --repo.
Standard library only.
"""

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

LABEL = "milestone-sync"
LABEL_COLOR = "6f42c1"
MARKER = "<!-- milestone-sync:id={} -->"
MARKER_RE = re.compile(r"<!-- milestone-sync:id=([a-z0-9-]+) -->")

HEADING_RE = re.compile(r"^## (v\d+\.\d+\.\d+) — (.+?)(?: \([^)]*\))?\s*$")
ITEM_RE = re.compile(r"^- \[( |x|X)\] (?:<!-- id:([a-z0-9-]+) --> )?(.*)$")
GOAL_RE = re.compile(r"^\*\*Obiettivo:\*\*\s*(.*)$")
BOLD_RE = re.compile(r"^\*\*(.+?)\*\*")


def parse(path):
    """Return (milestones, errors). Each milestone: {title, description, items: [...]}."""
    milestones, errors, seen = [], [], {}
    current, item = None, None
    with open(path, encoding="utf-8") as fh:
        lines = fh.read().splitlines()

    for n, line in enumerate(lines, 1):
        if line.startswith("## "):
            item = None
            m = HEADING_RE.match(line)
            current = None
            if m:
                current = {"title": f"{m.group(1)} — {m.group(2).strip()}", "description": "", "items": []}
                milestones.append(current)
            continue
        if current is None:
            continue

        g = GOAL_RE.match(line)
        if g and not current["description"]:
            current["description"] = g.group(1).strip()
            continue

        m = ITEM_RE.match(line)
        if m:
            done, item_id, text = m.group(1).lower() == "x", m.group(2), m.group(3).strip()
            if not item_id:
                errors.append(f"{path}:{n}: checklist item without `<!-- id:… -->`")
                item = None
                continue
            if item_id in seen:
                errors.append(f"{path}:{n}: duplicate id `{item_id}` (first at line {seen[item_id]})")
            seen[item_id] = n
            item = {"id": item_id, "done": done, "lines": [text], "milestone": current["title"]}
            current["items"].append(item)
            continue

        if item is not None and line.startswith("  ") and line.strip():
            item["lines"].append(line.strip())
        else:
            item = None

    for ms in milestones:
        for it in ms["items"]:
            text = " ".join(it["lines"])
            b = BOLD_RE.match(text)
            title = b.group(1) if b else text
            it["title"] = title.strip().rstrip(":").strip()[:120]
            it["text"] = text
    return milestones, errors


class GitHub:
    def __init__(self, repo, token, dry_run):
        self.base = f"https://api.github.com/repos/{repo}"
        self.repo, self.token, self.dry_run = repo, token, dry_run

    def call(self, method, path, body=None, write=False):
        if write and self.dry_run:
            return None
        url = path if path.startswith("https://") else self.base + path
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method, headers={
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "go-gemini-milestone-sync",
        })
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                raw = resp.read()
                link = resp.headers.get("Link", "")
                return (json.loads(raw) if raw else None), link
        except urllib.error.HTTPError as e:
            if e.code == 404 and method == "GET":
                return None, ""
            sys.exit(f"GitHub {method} {url}: HTTP {e.code} {e.read().decode(errors='replace')[:300]}")

    def get_all(self, path):
        out, url = [], self.base + path
        while url:
            page, link = self.call("GET", url)
            out.extend(page or [])
            nxt = re.search(r'<([^>]+)>;\s*rel="next"', link)
            url = nxt.group(1) if nxt else None
        return out


def token_from_env():
    for key in ("GITHUB_TOKEN", "GH_TOKEN"):
        if os.environ.get(key):
            return os.environ[key]
    try:
        return subprocess.run(["gh", "auth", "token"], capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        sys.exit("no token: set GITHUB_TOKEN or log in with `gh auth login`")


def issue_body(repo, it):
    return (
        f"{it['text']}\n\n---\n"
        f"Milestone: **{it['milestone']}** · generata da "
        f"[`docs/milestone.md`](https://github.com/{repo}/blob/main/docs/milestone.md) (id `{it['id']}`).\n"
        f"Modifica il file, non questa issue: il workflow *Milestone sync* la riallinea a ogni push su `main`.\n"
        f"{MARKER.format(it['id'])}\n"
    )


def sync(gh, milestones):
    actions = []

    def act(msg):
        actions.append(msg)
        print(("[dry-run] " if gh.dry_run else "") + msg)

    # label
    label, _ = gh.call("GET", f"/labels/{LABEL}")
    if label is None:
        act(f"create label `{LABEL}`")
        gh.call("POST", "/labels", {"name": LABEL, "color": LABEL_COLOR,
                                     "description": "Managed by docs/milestone.md"}, write=True)

    # milestones
    existing = {m["title"]: m for m in gh.get_all("/milestones?state=all&per_page=100")}
    numbers = {}
    for ms in milestones:
        items = ms["items"]
        want_state = "closed" if items and all(i["done"] for i in items) else "open"
        cur = existing.get(ms["title"])
        if cur is None:
            act(f"create milestone `{ms['title']}` ({want_state})")
            res = gh.call("POST", "/milestones", {"title": ms["title"], "description": ms["description"],
                                                   "state": want_state}, write=True)
            numbers[ms["title"]] = res[0]["number"] if res else None
            continue
        numbers[ms["title"]] = cur["number"]
        patch = {}
        if (cur.get("description") or "") != ms["description"]:
            patch["description"] = ms["description"]
        if cur["state"] != want_state:
            patch["state"] = want_state
        if patch:
            act(f"update milestone `{ms['title']}`: {', '.join(patch)}")
            gh.call("PATCH", f"/milestones/{cur['number']}", patch, write=True)

    wanted = {}
    for ms in milestones:
        for it in ms["items"]:
            wanted[it["id"]] = it

    # issues: list them all rather than filtering by label — the label-filtered listing lags a few
    # seconds behind writes, and a second run right after the first would create duplicates.
    by_id = {}
    by_title = {it["title"]: item_id for item_id, it in wanted.items()}
    for issue in sorted(gh.get_all("/issues?state=all&per_page=100"), key=lambda i: i["number"]):
        if "pull_request" in issue:
            continue
        m = MARKER_RE.search(issue.get("body") or "")
        item_id = m.group(1) if m else None
        if item_id is None and LABEL in [l["name"] for l in issue.get("labels", [])]:
            item_id = by_title.get(issue["title"])
        if item_id:
            by_id.setdefault(item_id, []).append(issue)

    managed = {}
    for item_id, issues in by_id.items():
        keep = next((i for i in issues if i["state"] == "open"), issues[0])
        managed[item_id] = keep
        for dup in issues:
            if dup is not keep and dup["state"] == "open":
                act(f"close issue #{dup['number']} [{item_id}] as duplicate of #{keep['number']}")
                gh.call("POST", f"/issues/{dup['number']}/comments",
                        {"body": f"Duplicate of #{keep['number']}."}, write=True)
                gh.call("PATCH", f"/issues/{dup['number']}",
                        {"state": "closed", "state_reason": "not_planned", "milestone": None}, write=True)

    for item_id, it in wanted.items():
        body = issue_body(gh.repo, it)
        number = numbers.get(it["milestone"])
        cur = managed.get(item_id)
        if cur is None:
            if it["done"]:
                continue  # never open an issue just to close it
            act(f"create issue `{it['title']}` [{item_id}] in `{it['milestone']}`")
            gh.call("POST", "/issues", {"title": it["title"], "body": body, "labels": [LABEL],
                                        "milestone": number}, write=True)
            continue
        patch = {}
        if cur["title"] != it["title"]:
            patch["title"] = it["title"]
        if (cur.get("body") or "").strip() != body.strip():
            patch["body"] = body
        if number is not None and (cur.get("milestone") or {}).get("number") != number:
            patch["milestone"] = number
        if it["done"] and cur["state"] == "open":
            patch.update(state="closed", state_reason="completed")
        elif not it["done"] and cur["state"] == "closed":
            print(f"warning: #{cur['number']} [{item_id}] is closed on GitHub but `[ ]` in the file — "
                  f"tick it in docs/milestone.md", file=sys.stderr)
        if patch:
            act(f"update issue #{cur['number']} [{item_id}]: {', '.join(patch)}")
            gh.call("PATCH", f"/issues/{cur['number']}", patch, write=True)

    for item_id, cur in managed.items():
        if item_id not in wanted and cur["state"] == "open":
            act(f"close issue #{cur['number']} [{item_id}] as not planned (removed from the file)")
            gh.call("PATCH", f"/issues/{cur['number']}", {"state": "closed", "state_reason": "not_planned"},
                    write=True)

    if not actions:
        print("already in sync")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--file", default="docs/milestone.md")
    ap.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", "Allan-Nava/go-gemini"))
    mode = ap.add_mutually_exclusive_group()
    mode.add_argument("--check", action="store_true", help="validate the file only, no network")
    mode.add_argument("--dry-run", action="store_true", help="print the plan, change nothing")
    args = ap.parse_args()

    milestones, errors = parse(args.file)
    for e in errors:
        print(f"error: {e}", file=sys.stderr)
    if errors:
        sys.exit(1)
    if not milestones:
        sys.exit(f"error: no `## vX.Y.Z — Title` heading in {args.file}")

    for ms in milestones:
        done = sum(i["done"] for i in ms["items"])
        print(f"{ms['title']}: {len(ms['items'])} items, {done} done")
    if args.check:
        return

    sync(GitHub(args.repo, token_from_env(), args.dry_run), milestones)


if __name__ == "__main__":
    main()
