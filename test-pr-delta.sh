#!/bin/bash
# Test script to verify PR delta calculation works correctly
# This simulates what the GitHub Action does

set -e

echo "Building kaizen..."
go build -o /tmp/kaizen ./cmd/kaizen

echo ""
echo "Analyzing base branch (main)..."
CURRENT_REF=$(git rev-parse HEAD)
git checkout main
/tmp/kaizen analyze --path=. --skip-churn --output=/tmp/base.json
git checkout "$CURRENT_REF"

echo ""
echo "Analyzing head branch (current)..."
/tmp/kaizen analyze --path=. --skip-churn --output=/tmp/head.json

echo ""
echo "Generating PR comment..."
/tmp/kaizen pr-comment --base-analysis=/tmp/base.json --head-analysis=/tmp/head.json

echo ""
echo "Extracting scores..."
python3 -c "
import json
with open('/tmp/base.json') as b, open('/tmp/head.json') as h:
    base = json.load(b)
    head = json.load(h)
    bs = base.get('score_report', {}).get('overall_score', 0)
    hs = head.get('score_report', {}).get('overall_score', 0)
    delta = hs - bs
    print(f'Base score: {bs:.1f}')
    print(f'Head score: {hs:.1f}')
    print(f'Delta: {delta:+.1f}')
    if delta == 0:
        print('WARNING: Delta is zero - this suggests the branches have identical code!')
    else:
        print('SUCCESS: Non-zero delta detected')
"
