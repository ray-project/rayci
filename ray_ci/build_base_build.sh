#!/bin/bash

set -euo pipefail

export DOCKER_BUILDKIT=1

if [[ "${RAY_REPO_DIR:-}" != "" ]]; then
  cd "$RAY_REPO_DIR" || true
fi

if [[ "${BUILDKITE_COMMIT:-HEAD}" == "HEAD" ]]; then
  BUILDKITE_COMMIT="$(git rev-parse HEAD)"
fi
echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"

ECR_BASE_REPO="${ECR_BASE_REPO:-029272617770.dkr.ecr.us-west-2.amazonaws.com/ci_base_images}"

DOCKER_IMAGE_BASE_TEST=$ECR_BASE_REPO:oss-ci-base_test_$BUILDKITE_COMMIT
DOCKER_IMAGE_BASE_BUILD=$ECR_BASE_REPO:oss-ci-base_build_$BUILDKITE_COMMIT
DOCKER_IMAGE_BASE_ML=$ECR_BASE_REPO:oss-ci-base_ml_$BUILDKITE_COMMIT

# DOCKER_IMAGE_BASE_TEST is used as build arg
export DOCKER_IMAGE_BASE_TEST

echo "--- :docker: Building base dependency image for TESTS :python:"

docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BASE_TEST" \
  -f ci/docker/base.test.Dockerfile .

echo "--- :docker: Building base dependency image for BUILDS :gear:"

docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_BUILD" \
  -f ci/docker/base.build.Dockerfile .

echo "--- :docker: Building base dependency image for ML :airplane:"

docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_ML" \
  -f ci/docker/base.ml.Dockerfile .


if [[ "${NO_PUSH:-}" == "1" ]]; then
  echo "--- :exclamation: Not pushing the image as this is a local build only!"
  exit 0
fi

echo "--- :arrow_up: Pushing docker images to ECR"

# Convert / into _
if [[ "${BUILDKITE_PULL_REQUEST_BASE_BRANCH:-}" != "" ]]; then
  # In PRs, use the BUILDKITE_PULL_REQUEST_BASE_BRANCH
  BRANCH_NAME="${BUILDKITE_PULL_REQUEST_BASE_BRANCH//\//_}"
elif [[ "${BUILDKITE_BRANCH:-}" != "" ]]; then
  # In branches, use the BUILDKITE_BRANCH
  BRANCH_NAME="${BUILDKITE_BRANCH//\//_}"
else
  BRANCH_NAME="dev"
fi

DOCKER_IMAGE_TAG_TEST="${ECR_BASE_REPO}:oss-ci-base_test_latest_${BRANCH_NAME}"
DOCKER_IMAGE_TAG_BUILD="${ECR_BASE_REPO}:oss-ci-base_build_latest_${BRANCH_NAME}"
DOCKER_IMAGE_TAG_ML="${ECR_BASE_REPO}:oss-ci-base_ml_latest_${BRANCH_NAME}"

echo "--- Push ci-base_test"
docker push "$DOCKER_IMAGE_BASE_TEST"

echo "--- Push ci-base_build"
docker push "$DOCKER_IMAGE_BASE_BUILD"

echo "--- Push ci-base_ml"
docker push "$DOCKER_IMAGE_BASE_ML"

echo "--- Tagging aliases"

docker tag "$DOCKER_IMAGE_BASE_BUILD" "$DOCKER_IMAGE_TAG_BUILD"
docker push "$DOCKER_IMAGE_TAG_BUILD"

docker tag "$DOCKER_IMAGE_BASE_TEST" "$DOCKER_IMAGE_TAG_TEST"
docker push "$DOCKER_IMAGE_TAG_TEST"

docker tag "$DOCKER_IMAGE_BASE_ML" "$DOCKER_IMAGE_TAG_ML"
docker push "$DOCKER_IMAGE_TAG_ML"
