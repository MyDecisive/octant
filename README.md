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
