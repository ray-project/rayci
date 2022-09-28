#!/bin/bash
set -e

export DOCKER_BUILDKIT=1

cd "$RAY_REPO_DIR" || true

if [ "$BUILDKITE_COMMIT" = "HEAD" ]; then
  export BUILDKITE_COMMIT=$(git log -1 --format="%H")
  echo "Resolved BUILDKITE_COMMIT to $BUILDKITE_COMMIT"
fi

# Convert / into _
if [ -z "${BUILDKITE_PULL_REQUEST_BASE_BRANCH-}" ]; then
  # In branches, use the BUILDKITE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}
else
  # In PRs, use the BUILDKITE_PULL_REQUEST_BASE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_PULL_REQUEST_BASE_BRANCH/\//_}
fi

export DOCKER_IMAGE_BASE_TEST=$ECR_BASE_REPO:oss-ci-base_test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_TEST=$ECR_BASE_REPO:oss-ci-base_test_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BASE_BUILD=$ECR_BASE_REPO:oss-ci-base_build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_BUILD=$ECR_BASE_REPO:oss-ci-base_build_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BASE_ML=$ECR_BASE_REPO:oss-ci-base_ml_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_ML=$ECR_BASE_REPO:oss-ci-base_ml_latest_$BUILDKITE_BRANCH_CLEAN


echo "--- :docker: Building base dependency image for TESTS :python:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BASE_TEST" \
  -t "$DOCKER_IMAGE_TAG_TEST" \
  -f ci/docker/base.test.Dockerfile .

echo "--- :docker: Building base dependency image for BUILDS :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_BUILD" \
  -t "$DOCKER_IMAGE_TAG_BUILD" \
  -f ci/docker/base.build.Dockerfile .

echo "--- :docker: Building base dependency image for ML :airplane:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_ML" \
  -t "$DOCKER_IMAGE_TAG_ML" \
  -f ci/docker/base.ml.Dockerfile .

date +"%Y-%m-%d %H:%M:%S"

if [ "${NO_PUSH}" = "1" ]; then
  echo "--- :exclamation: Not pushing the image as this is a local build only!"
  exit 0
fi

echo "--- :arrow_up: Pushing docker images to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BASE_BUILD"
time docker push "$DOCKER_IMAGE_TAG_BUILD"

time docker push "$DOCKER_IMAGE_BASE_TEST"
time docker push "$DOCKER_IMAGE_TAG_TEST"

time docker push "$DOCKER_IMAGE_BASE_ML"
time docker push "$DOCKER_IMAGE_TAG_ML"

date +"%Y-%m-%d %H:%M:%S"
