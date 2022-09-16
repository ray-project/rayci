set -e

cd "$RAY_REPO_DIR" || true

export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}

export DOCKER_IMAGE_BASE_TEST=$ECR_BASE_REPO:oss-ci-base_test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_TEST=$ECR_BASE_REPO:oss-ci-base_test_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BASE_BUILD=$ECR_BASE_REPO:oss-ci-base_build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_BUILD=$ECR_BASE_REPO:oss-ci-base_build_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BASE_ML=$ECR_BASE_REPO:oss-ci-base_ml_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_ML=$ECR_BASE_REPO:oss-ci-base_ml_latest_$BUILDKITE_BRANCH_CLEAN


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

echo "--- :docker: Building base dependency image for ML :airplane:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_BASE_ML" \
  -t "$DOCKER_IMAGE_TAG_ML" \
  -f ci/docker/Dockerfile.base_ml .

date +"%Y-%m-%d %H:%M:%S"

if [ "${NO_PUSH}" = "1 "]; then
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
