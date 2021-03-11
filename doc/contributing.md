# Contributing

## Changelog

Every change (feature, bug fix, etc.) should be made in a PR that includes an update to the `CHANGELOG_PENDING.md` file which tracks the set of changes that will be included in the next release.

Changelog entries should be formatted as follows:

```
- \#xxx Description of the change (@contributor)
```

`xxx` is the PR number (if for whatever reason a PR number cannot be used the issue number is acceptable) and `contributor` is the author of the change. The full link to the PR is not necessary because it will automatically be added when a release is created, but make sure to include the backslash and pound i.e. `\#2313`.

Changelog entries should be classified based on the `livepeer` mode of operation that they pertain to i.e. General, Broadcaster, Orchestrator, Transcoder.

Breaking changes should be documented in the "Breaking changes" section. Any changes that involve pending deprecations (i.e. a flag will be removed in the next release) should be documented in the "Upcoming changes" section.