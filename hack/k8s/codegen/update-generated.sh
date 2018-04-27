#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

DOCKER_REPO_ROOT="/go/src/github.com/nuance-mobility/zookeeper-operator"
IMAGE=${IMAGE:-"gcr.io/coreos-k8s-scale-testing/codegen"}

docker run --rm \
  -v "$PWD":"$DOCKER_REPO_ROOT" \
  -w "$DOCKER_REPO_ROOT" \
  "$IMAGE" \
  "/go/src/k8s.io/code-generator/generate-groups.sh"  \
  "all" \
  "github.com/nuance-mobility/zookeeper-operator/pkg/generated" \
  "github.com/nuance-mobility/zookeeper-operator/pkg/apis" \
  "zookeeper:v1alpha1" \
  --go-header-file "./hack/k8s/codegen/boilerplate.go.txt" \
  $@
