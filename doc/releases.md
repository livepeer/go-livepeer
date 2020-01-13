# go-livepeer release process

We presently use three branches for go-livepeer releases.

#### master

Built in CI with go build `-tags dev`, releasing binaries that may only connect to private networks. Published to Docker Hub as `livepeer/go-livepeer:master`.

#### rinkeby

Cut off of master periodically when we're comfortable that it's reasonably stable. Built in CI with `-tags rinkeby`, so it can connect to the Rinkeby test network. Published to Docker Hub as `livepeer/go-livepeer:rinkeby`.

#### mainnet

Cut off of `rinkeby` when we want to do a proper mainnet release. Built in CI with `-tags mainnet`, giving it the ability to connect to the Ethereum mainnet.

This is the only branch that receives proper semver updates; all releases should have a tag of the form `vMAJOR.MINOR.PATCH`. Published to Docker Hub as `livepeer/go-livepeer:mainnet` and e.g. `livepeer/go-livepeer:0.5.3`.

### Cutting a mainnet release of go-livepeer

This process will vary somewhat based on the particular states of the branches, whether we're doing a hotfix, etc. But the overall steps are the same. First, merge changes into the mainnet branch and make the release commit:

```bash
git checkout mainnet
git merge --ff-only rinkeby
echo -n '0.5.2' > VERSION
git commit -am 'release v0.5.2'
git push --atomic origin mainnet v0.5.2
```

Then, merge that version bump back to rinkeby and master:

```bash
git checkout rinkeby
git merge --ff-only mainnet
git push origin rinkeby
git checkout master
git merge --ff-only mainnet
git push origin master
```

If there's different commits on those branches then we'd omit the --ff-only flag, but the cleanest possible scenario after a mainnet release is that all three branches are pointed at the same ref.
