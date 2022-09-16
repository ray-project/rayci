set -e

cd "$RAY_REPO_DIR" || true

export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}

export DOCKER_IMAGE_BASE_GPU=$ECR_REPO:oss-ci-base_gpu_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_GPU=$ECR_REPO:oss-ci-base_gpu_latest_$BUILDKITE_BRANCH_CLEAN

echo "--- :docker: Building base dependency image for GPU tests :tv:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BASE_GPU" \
  -t "$DOCKER_IMAGE_TAG_GPU" \
  -f ci/docker/Dockerfile.base_gpu .

date +"%Y-%m-%d %H:%M:%S"

if [ "${NO_PUSH}" = "1 "]; then
  echo "--- :exclamation: Not pushing the image as this is a local build only!"
  exit 0
fi

echo "--- :arrow_up: Pushing docker image to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BASE_GPU"
time docker push "$DOCKER_IMAGE_TAG_GPU"

date +"%Y-%m-%d %H:%M:%S"
