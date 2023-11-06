# go-livepeer release process

### Branches

We presently use three branches for go-livepeer releases.

#### master

This branch is compatible with contracts on Rinkeby and mainnet. Code committed to this branch MUST NOT break contract compatibility on Rinkeby or mainnet.

Releases are cut from this branch. All releases should have a tag of the form `vMAJOR.MINOR.PATCH`.

Built in CI with `-tags mainnet` so resulting binaries can connect to private networks, Rinkeby and mainnet. Published to Docker Hub as `livepeer/go-livepeer:master` and e.g. `livepeer/go-livepeer:0.5.3`.

#### rinkeby

This branch is compatible with contracts on Rinkeby, but may be incompatible with contracts on mainnet. Code committed to this branch MUST NOT break contract compatibility on Rinkeby, but may break contract compatibility on mainnet. This branch can be merged into `master` when it is compatible with contracts on mainnet.

Built in CI with `-tags rinkeby` so resulting binaries can connect to private networks or Rinkeby. Published to Docker Hub as `livepeer/go-livepeer:rinkeby`.

#### dev 

This branch may be incompatible with contracts on Rinkeby and mainnet. Code committed to this branch can break contract compatibility with Rinkeby or mainnet. This branch can be merged into `rinkeby` when it becomes compatible with contracts on Rinkeby.

Built in CI with `-tags dev` so resulting binaries can only connect to private networks. Published to DockerHub as `livepeer/go-livepeer:dev`.

### Contract Compatible Changes

If changes are compatible with contracts on Rinkeby and mainnet then they can be committed directly to the `master` branch.

### Contract Incompatible Changes

Suppose certain changes depend on a new contract that has not been deployed on Rinkeby or mainnet yet or a contract upgrade that has not been executed on Rinkeby or mainnet yet. The steps to land these changes on the `master` branch would be:

1. Deploy the contract(s) on a private network
2. Test the changes on the private network
3. Merge the PR(s) into `dev`
4. Deploy/upgrade the contract(s) on Rinkeby
5. Merge `dev` into `rinkeby` via PR
6. Test the changes on Rinkeby
7. Deploy/upgrade the contract(s) on mainnet
8. Merge `rinkeby` into `master` via PR

Note that step 3 and on do not need to be executed after every single PR merge in `dev`. Multiple PRs can be merged to `dev` before executing step 3 and on. 

### Release flow

Double-check that `master` is deployed to staging and that it works correctly.

### Cutting a mainnet release of go-livepeer

Create the release commit on a branch:

1. Checkout a release branch

    ```bash
    git checkout -b release-0.5.2
    ```

2. `echo -n '0.5.2' > VERSION`

3. Update the changelog

    - Copy all entries from `CHANGELOG_PENDING.md` to the top of `CHANGELOG.md` and update `vX.X` to the new version number
    - Run `go run cmd/scripts/linkify_changelog.go CHANGELOG.md` to add links for all PRs
    - Reset `CHANGELOG_PENDING.md`

4. `git commit -am 'release v0.5.2'`

5. `git push -u origin release-0.5.2`

6. Merge the release commit into `master` via PR. Then, push the release tag up.

```bash
git checkout master
git pull
git log # Confirm that no new changes came into master while you were doing the previous steps
git tag v0.5.2
git push origin v0.5.2
```

7. Once the CI (Github Actions) process completes, you should see your release at https://github.com/livepeer/go-livepeer/releases. Fix up the release notes to be more human-friendly, using previous releases as a guide.

8. Update commit hash, version and checksum for Homebrew as per https://github.com/livepeer/homebrew-tap/pull/5
9. Announce the release on Discord in #network-announcements
