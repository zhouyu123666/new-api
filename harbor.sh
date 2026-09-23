#!/usr/bin/env bash

export HARBOR_HOST="registry.intsig.net"
export HARBOR_PROJECT="noc_maas"
export HARBOR_REPOSITORY="new-api"

export GIT_BRANCH="$(git branch --show-current)"
export GIT_SHA="$(git rev-parse --short=12 HEAD)"
export IMAGE_TAG="${GIT_BRANCH}-${GIT_SHA}"
export IMAGE="${HARBOR_HOST}/${HARBOR_PROJECT}/${HARBOR_REPOSITORY}:${IMAGE_TAG}"

echo "$IMAGE"

read -r -s -p "Harbor password: " HARBOR_PASSWORD
printf '\n'
printf '%s' "$HARBOR_PASSWORD" | docker login "$HARBOR_HOST" \
  --username 'robot$noc_maas_robot' \
  --password-stdin
unset HARBOR_PASSWORD

docker build --pull --progress=plain --tag "$IMAGE" .
docker push "$IMAGE"