#!/bin/bash
set -e

cd "$RAY_REPO_DIR" || true

python3 -m pip install -U click pyyaml

export $(python3 ci/pipeline/determine_tests_to_run.py)
env

echo "--- :rocket: Launching Mac OS tests :gear:"
echo "Kicking off the full Mac OS pipeline"

export RUNNER_QUEUE_ARM64_MEDIUM="$RUNNER_QUEUE_MAC_ARM64"

python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --queue "$RUNNER_QUEUE_DEFAULT" \
  --base-step-file "${PIPELINE_REPO_DIR}/ray_ci/step_macos.json" \
  "./.buildkite/pipeline.macos.yml" > pipeline.txt

cat pipeline.txt
cat pipeline.txt | buildkite-agent upload --no-interpolation
