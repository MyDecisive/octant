[![Chores](https://github.com/mydecisive/octant/actions/workflows/chores.yml/badge.svg)](https://github.com/mydecisive/octant/actions/workflows/chores.yml)
[![codecov](https://codecov.io/gh/MyDecisive/octant/graph/badge.svg?token=UPHRBSXOON)](https://codecov.io/gh/MyDecisive/octant)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/octant)](https://artifacthub.io/packages/search?repo=octant)

# MDAI Octant

Check out the [octant](https://docs.mydecisive.ai/) docs.

## Building Octant
### Pre-Requisites
Octant communicates with a kubernetes cluster to manage objects that it needs for various operations. If you are running
octant **_outside_** of a cluster, ensure that you have a valid kubernetes config (`~/.kube/config`) set to your local cluster context.
When run inside a cluster, it will have everything it needs due to the Role/RoleBinding octant sets up.
```shell
kubectl config current-context
```
### Building and running just the Octant API:
Octant API runs on port `50051`, so the easiest way to integrate with it for localdev is to set the `baseUrl` in the octant
typescript client to `localhost:50051`.<br>
```shell
go build -trimpath -ldflags="-w -s" -o octant ./cmd/octant
./octant
```

### Building the "full" webapp:
This requires having the `octant-ui` repo also checked out. This method is desired if you 
```shell
# From the octant-ui repo base dir
npm install
npm run build
cp -R dist /path/to/octant/web/
# Then, from the octant repo base dir
go build -trimpath -tags webapp -ldflags="-w -s" -o octant ./cmd/octant
```

### Building the docker image and deploying to the local cluster
NOTE: the docker build defaults to the `ghcr.io/mydecisive/octant-ui:latest` image for `octant-ui`
```shell
make docker-build
# to override the octant-ui image with a locally built image
OCTANT_UI_IMAGE=octant-ui:0.1.2 make docker-build
```
Then, load the image to the kind cluster, package a helm release, and deploy it.
```shell
# replace `local/octant-ui` with whatever you named your image
kind load docker-image local/octant-ui:latest --name mdai
# helm package and deploy
make helm-package
helm upgrade octant ./octant-0.1.2.tgz --install
```
