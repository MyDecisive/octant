[![Chores](https://github.com/mydecisive/octant/actions/workflows/chores.yml/badge.svg)](https://github.com/mydecisive/octant/actions/workflows/chores.yml)
[![codecov](https://codecov.io/gh/MyDecisive/octant/graph/badge.svg?token=UPHRBSXOON)](https://codecov.io/gh/MyDecisive/octant)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/octant)](https://artifacthub.io/packages/search?repo=octant)

# MDAI Octant

Check out the [octant](https://docs.mydecisive.ai/) docs.

## Building Octant
Building just the Octant API
```shell
go build -trimpath -ldflags="-w -s" -o octant ./cmd/octant
```

Building the "full" webapp requires having the `octant-ui` repo also checked out.
```shell
# From the octant-ui repo base dir
npm install
npm run build
cp -R dist /path/to/octant/web/
```
```shell
# Then, from the octant repo base dir
go build -trimpath -tags webapp -ldflags="-w -s" -o octant ./cmd/octant
```

Building the docker image uses the [git repo build context](https://docs.docker.com/build/concepts/context/#git-repositories) to pull the webapp files in to build from.
