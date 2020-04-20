# go-livepeer release process

### Branches

We presently use three branches for go-livepeer releases.

#### master

Built in CI with go build `-tags dev`, releasing binaries that may only connect to private networks. Published to Docker Hub as `livepeer/go-livepeer:master`.

#### rinkeby

Cut off of master periodically when we're comfortable that it's reasonably stable. Built in CI with `-tags rinkeby`, so it can connect to the Rinkeby test network. Published to Docker Hub as `livepeer/go-livepeer:rinkeby`.

#### mainnet

Cut off of `rinkeby` when we want to do a proper mainnet release. Built in CI with `-tags mainnet`, giving it the ability to connect to the Ethereum mainnet.

This is the only branch that receives proper semver updates; all releases should have a tag of the form `vMAJOR.MINOR.PATCH`. Published to Docker Hub as `livepeer/go-livepeer:mainnet` and e.g. `livepeer/go-livepeer:0.5.3`.

### Release flow

Once all planned code updates are merged into `master`, we create release candidate binaries that can connect to Rinkeby:

```bash
git checkout rinkeby
git merge --ff-only master 
```

We use the release candidate binaries for internal testing on Rinkeby. During this stage, we

1. Roll out the binaries to internal Rinkeby infrastructure
2. Check that user facing bugs described in closed bug reports cannot be reproduced
3. Check that user facing features are working as expected
4. Run stream tests that involve sending/monitoring streams sent into broadcasters that are then routed to one or many orchestrators

The goals of this stage are:

1. Test the release candidate on internal infrastructure that can be easily monitored
2. Make sure that code updates did not introduce any obvious regressions
3. Perform manual QA for user facing changes

Once we complete this stage, we prepare a mainnet release.

### Cutting a mainnet release of go-livepeer

This process will vary somewhat based on the particular states of the branches, whether we're doing a hotfix, etc. But the overall steps are the same. First, make the release commit on a branch:

```bash
git checkout -b release-0.5.2 
echo -n '0.5.2' > VERSION
git commit -am 'release v0.5.2'
git push -u origin release-0.5.2
```

Merge the release commit into master via PR. Then, merge the version bump into rinkeby:

```bash
git checkout rinkeby
git merge --ff-only master 
git push origin rinkeby
```

Wait just a moment for the Rinkeby build to start so that it doesn't build with the mainnet release tag. Then, merge the version bump into mainnet and create the release tag:

```bash
git checkout mainnet
git merge --ff-only rinkeby
git tag v0.5.2
git push --atomic origin mainnet v0.5.2
```

If there's different commits on those branches then we'd omit the --ff-only flag, but the cleanest possible scenario after a mainnet release is that all three branches are pointed at the same ref.
