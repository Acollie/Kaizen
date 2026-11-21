#!/usr/bin/env python3
"""Summarise churn from a kaizen analysis JSON file.

Usage: churn_report.py kaizen-results.json [--top N]
"""

import argparse
import json
import statistics
import sys


def _percentile(values, fraction):
    """Return the value at the given fraction (0-1) of the sorted list."""
    if not values:
        return 0
    ordered = sorted(values)
    index = min(len(ordered) - 1, int(round(fraction * (len(ordered) - 1))))
    return ordered[index]


def load_files(path):
    with open(path, encoding="utf-8") as handle:
        data = json.load(handle)
    return data.get("files", [])


def summarize_churn(files, top=10):
    """Return churn statistics and the files that change most often."""
    churned = [f for f in files if f.get("churn")]
    commits = [f["churn"]["total_commits"] for f in churned]
    if not commits:
        return {"files": 0, "median": 0, "p90": 0, "top": []}

    by_commits = sorted(churned, key=lambda f: f["churn"]["total_commits"], reverse=True)
    top_files = []
    for entry in by_commits[:top]:
        churn = entry["churn"]
        top_files.append({
            "path": entry["path"],
            "commits": churn["total_commits"],
            "authors": churn.get("author_count", 0),
            "changes_per_commit": churn["total_changes"] / churn["total_commits"],
        })

    return {
        "files": len(churned),
        "median": statistics.median(commits),
        "p90": _percentile(commits, 0.9),
        "top": top_files,
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("results")
    parser.add_argument("--top", type=int, default=10)
    args = parser.parse_args(argv)

    summary = summarize_churn(load_files(args.results), top=args.top)
    json.dump(summary, sys.stdout, indent=2)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
