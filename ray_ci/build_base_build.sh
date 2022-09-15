set -e

cd "$RAY_REPO_DIR" || true

export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}

export DOCKER_IMAGE_BASE_TEST=$ECR_REPO:oss-ci-base_test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_TEST=$ECR_REPO:oss-ci-base_test_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BASE_BUILD=$ECR_REPO:oss-ci-base_build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_BUILD=$ECR_REPO:oss-ci-base_build_$BUILDKITE_COMMIT

echo "--- :docker: Building base dependency image for TESTS :python:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BASE_TEST" \
  -t "$DOCKER_IMAGE_TAG_TEST" \
  -f ci/docker/Dockerfile.base_test .

echo "--- :docker: Building base dependency image for BUILDS :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_BUILD" \
  -t "$DOCKER_IMAGE_TAG_BUILD" \
  -f ci/docker/Dockerfile.base_build .

date +"%Y-%m-%d %H:%M:%S"

echo "--- :arrow_up: Pushing docker images to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BASE_BUILD"
time docker push "$DOCKER_IMAGE_TAG_BUILD"

time docker push "$DOCKER_IMAGE_BASE_TEST"
time docker push "$DOCKER_IMAGE_TAG_TEST"

date +"%Y-%m-%d %H:%M:%S"
