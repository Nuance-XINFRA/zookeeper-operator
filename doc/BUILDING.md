Build the operator:
- `dep ensure`
- `./hack/k8s/codegen/update-generated.sh`
- `./hack/build/operator/build`
- `docker build -f ./hack/build/operator/Dockerfile .`