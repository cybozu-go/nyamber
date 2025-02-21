# Maintenance

## How to update supported Kubernetes

Nyamber supports the three latest Kubernetes versions.
If a new Kubernetes version is released, please update the followings:

### 1. Update supported kubernetes and dependencies versions

- Kubernetes versions:
  - `k8s-version` in [.github/workflows/ci.yaml](/.github/workflows/ci.yaml): You can check the available versions at <https://hub.docker.com/r/kindest/node/tags>.
  - "Supported software" in [README.md](/README.md)
- Kubernetes tools versions:
  - Update `KUSTOMIZE_VERSION` in [Makefile](/Makefile) to the latest version from <https://github.com/kubernetes-sigs/kustomize/releases>.
  - Update `E2ETEST_K8S_VERSION` in [e2e/Makefile](/e2e/Makefile) to the latest supported version of kubernetes.
- After saving the changes above, update `ENVTEST_K8S_VERSION` in [Makefile](/Makefile) to the latest patch version among the latest supported kubernetes minor versions listed by running `make setup && bin/setup-envtest list` at the root of this repository. If the latest minor supported version is `1.30.Z`, find `1.30.Z+` from the output but not `1.31.Z`.
- Run `aqua update` to update tools in [aqua.yaml](/aqua.yaml). Then, manually align the minor versions of kubernetes and kubectl in `aqua.yaml`. If the minor version of `kubernetes/kubectl` in `aqua.yaml` precedes the latest supported minor versions of kubernetes, adjust it to match.  
  _e.g._, If `kubernetes/kubectl` is `1.31.Z` and the latest supported version is `1.30.Z`, modify `kubernetes/kubectl` version to `1.30.Z`.
- Other tools versions:
  - Update `PLACEMAT_VERSION` in [Dockerfile.runner](/Dockerfile.runner) to the latest version from <https://github.com/cybozu-go/placemat/releases>.
  - Update `CONTROLLER_TOOLS_VERSION` in [Makefile](/Makefile) to the latest version from <https://github.com/kubernetes-sigs/controller-tools/releases>.
  - Update `CERT_MANAGER_VERSION` in [e2e/Makefile](/e2e/Makefile) to the latest version from <https://github.com/cert-manager/cert-manager/releases>.
- Other dependencies versions:
  - Update actions in [.github/workflows/ci.yaml](/.github/workflows/ci.yaml) and [.github/workflows/release.yaml](/.github/workflows/release.yaml)
  - Update `ghcr.io/cybozu/golang` image and `GO_VERSION` in [Dockerfile](/Dockerfile) and [Dockerfile.runner](/Dockerfile.runner) to the latest version from <https://github.com/cybozu/neco-containers/pkgs/container/golang>.
- `go.mod` and `go.sum`:
  - Run [update-gomod](https://github.com/masa213f/tools/tree/main/cmd/update-gomod).

If Kubernetes or related APIs have changed, please update the relevant source code accordingly.

### 2. Update nyamber by running `make`

You can update nyamber by running the following `make` commands:

```sh
make setup
make manifests
make build
```

### 3. Fix test code if tests fail

After pushing the change, if the CI fails, fix the tests and push the changes again.

_e.g._, <https://github.com/cybozu-go/nyamber/pull/59>

### 4. Release the new version

After merging the changes above, follow the procedures written in [docs/release.md](/docs/release.md) and release the new version.

_e.g._, <https://github.com/cybozu-go/nyamber/pull/60>
