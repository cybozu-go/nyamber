load('ext://restart_process', 'docker_build_with_restart')

CONTROLLER_DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/manager /
CMD ["/manager"]
'''

def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def controller_binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager cmd/nyamber-controller/main.go'

def entrypoint_binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/entrypoint cmd/entrypoint/main.go'

# Generate manifests and go files
local_resource('make manifests', manifests(), deps=["api", "controllers", "hooks"], ignore=['*/*/zz_generated.deepcopy.go'])
local_resource('make generate', generate(), deps=["api", "hooks"], ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy CRD
local_resource(
    'CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"],
    ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy manager
watch_file('./config/')
k8s_yaml(kustomize('./config/dev'))

local_resource(
    'Watch & Compile (nyamber controller)', generate() + controller_binary(), deps=['controllers', 'api', 'pkg', 'hooks', 'cmd'],
    ignore=['*/*/zz_generated.deepcopy.go'])

local_resource('Watch & Compile (entrypoint)', entrypoint_binary(), deps=['pkg', 'cmd'])

docker_build_with_restart(
    'nyamber-controller:dev', '.',
    dockerfile_contents=CONTROLLER_DOCKERFILE,
    entrypoint=['/manager'],
    only=['./bin/manager'],
    live_update=[
        sync('./bin/manager', '/manager'),
    ]
)

local_resource('runner image',
    'docker build -t localhost:5151/nyamber-runner:dev -f Dockerfile.runner .; docker push localhost:5151/nyamber-runner:dev',
    deps=['Dockerfile.runner', './bin/entrypoint'] )

local_resource(
    'Sample', 'kubectl apply -f ./config/samples/nyamber_v1beta1_virtualdc.yaml',
    deps=["./config/samples/nyamber_v1beta1_virtualdc.yaml"])
