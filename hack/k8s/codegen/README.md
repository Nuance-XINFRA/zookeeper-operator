## How to use codegen

Dependencies:
- Docker

In repo root dir, run:

```sh
./hack/k8s/codegen/update-generated.sh
```

It should print:

```
Generating deepcopy funcs
Generating clientset for zookeeper:v1alpha1 at github.com/nuance-mobility/zookeeper-operator/pkg/generated/clientset
Generating listers for zookeeper:v1alpha1 at github.com/nuance-mobility/zookeeper-operator/pkg/generated/listers
Generating informers for zookeeper:v1alpha1 at github.com/nuance-mobility/zookeeper-operator/pkg/generated/informers
```
