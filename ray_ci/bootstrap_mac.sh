#!/bin/bash
set -e

cd "$RAY_REPO_DIR" || true

if [[ ! -f .buildkite/pipeline.macos.yml ]]; then
  echo "Pipeline file not found"
  exit 0
fi

python3 -m pip install -U click pyyaml

export $(python3 ci/pipeline/determine_tests_to_run.py)
env

echo "--- :rocket: Launching Mac OS tests :gear:"
echo "Kicking off the full Mac OS pipeline"

# Fix: pr runners currently don't have access to the artifact bucket
unset BUCKET_PATH || true

python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --queue "$RUNNER_QUEUE_DEFAULT" \
  --base-step-file "${PIPELINE_REPO_DIR}/ray_ci/step_macos.json" \
  "./.buildkite/pipeline.macos.yml" > pipeline.txt

cat pipeline.txt
cat pipeline.txt | buildkite-agent pipeline upload --no-interpolation
