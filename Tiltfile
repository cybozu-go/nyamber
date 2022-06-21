load('ext://restart_process', 'docker_build_with_restart')

DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/manager /
CMD ["/manager"]
'''

def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager main.go'

# Generate manifests and go files
local_resource('make manifests', manifests(), deps=["api", "controllers"], ignore=['*/*/zz_generated.deepcopy.go'])
local_resource('make generate', generate(), deps=["api"], ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy CRD
local_resource(
    'CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"],
    ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy manager
k8s_yaml(kustomize('./config/dev'))

local_resource(
    'Watch & Compile', generate() + binary(), deps=['controllers', 'api', 'pkg', 'main.go'],
    ignore=['*/*/zz_generated.deepcopy.go'])

docker_build_with_restart(
    'nyamber-controller:dev', '.',
    dockerfile_contents=DOCKERFILE,
    entrypoint=['/manager'],
    only=['./bin/manager'],
    live_update=[
        sync('./bin/manager', '/manager'),
    ]
)

local_resource(
    'Sample', 'kubectl apply -f ./config/samples/nyamber_v1beta1_virtualdc.yaml',
    deps=["./config/samples/nyamber_v1beta1_virtualdc.yaml"])
