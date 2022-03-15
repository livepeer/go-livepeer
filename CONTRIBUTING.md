# Contributing to go-livepeer

Hey there, potential contributor!

Thanks for your interest in this project. Every contribution is welcome and
appreciated. We're super excited to help you to get started 😎

> **Note:** If you still have questions after reading through this guide,
> [open an issue](https://github.com/livepeer/go-livepeer/issues) or
> [talk to us on Discord](https://discordapp.com/invite/7wRSUGX).

### How You Can Help

go-livepeer contributions will generally fall into one of the following
categories:

#### 📖 Updating documentation

This could be as simple as adding some extra notes to a README.md file, or as
complex as creating some new `package.json` scripts to generate docs. Either
way, we'd really really love your help with this 💖. Look for
[open documentation issues](https://github.com/livepeer/go-livepeer/labels/type%3A%20documentation),
create your own, or submit a PR with the updates you want to see.

If you're interested in updating user (e.g., node operator) documentation, please refer to [the relevant documentation on livepeer.org](https://docs.livepeer.org/contributing/overview).

#### 💬 Getting involved in issues
As a starting point, check out the issues that we've labeled as 
[help wanted](https://github.com/livepeer/go-livepeer/labels/help%20wanted)
and
[good first issues](https://github.com/livepeer/go-livepeer/labels/good%20first%20issue).

Many issues are open discussions. Feel free to add your own concerns, ideas, and
workarounds. If you don't see what you're looking for, you can always open a new
issue. 

#### 🐛 Fixing bugs, 🕶️ adding feature/enhancements, or 👌 improving code quality

If you're into coding, maybe try fixing a
[bug](https://github.com/livepeer/go-livepeer/issues?q=is%3Aissue+is%3Aopen+label%3A%22type%3A+bug%22)
or taking on a
[feature request](https://github.com/livepeer/go-livepeer/issues?q=is%3Aissue+is%3Aopen+label%3A%22type%3A+feature%22+).

If picking up issues isn't your thing, no worries -- you can always add more
tests to improve coverage or refactor code to increase maintainability. 

> Note: Bonus points if you can delete code instead of adding it! 👾

#### 🛠️ Updating scripts and tooling

We want to make sure go-livepeer contributors have a pleasant developer
experience (DX). The tools we use to code continually change and improve. If you
see ways to reduce the amount of repetition or stress when it comes to coding in
this project, feel free to create an issue and/or PR to discuss. Let's continue
to improve this codebase for everyone.

> Note: These changes generally affect multiple packages, so you'll probably
> want to be familiar with each project's layout and conventions. Because of
> this additional cognitive load, you may not want to begin here for you first
> contribution.

#### Commits

- We generally prefer to logically group changes into individual commits as much as possible and like to follow these [guidelines](https://github.com/lightningnetwork/lnd/blob/master/docs/code_contribution_guidelines.md#ideal-git-commit-structure) for commit structure
- We like to use [fixup commits and auto-squashing when rebasing](https://thoughtbot.com/blog/autosquashing-git-commits) during the PR review process to a) make it easy to review incremental changes and b) make it easy to logically group changes into individual commits
- We like to use descriptive commit messages following these [guidelines](https://github.com/lightningnetwork/lnd/blob/master/docs/code_contribution_guidelines.md#model-git-commit-messages). Additionally, we like to prefix commit titles with the package/functionality that the commit updates (i.e. `eth: ...`) as described [here](https://github.com/lightningnetwork/lnd/blob/master/docs/code_contribution_guidelines.md#ideal-git-commit-structure)

## FAQ

### How much do I need to know about peer-to-peer/livestreaming/go/etc to be an effective contributor?

We expect a rich mixture of commits, conversation, support, and review. Adding documentation or opening issues are incredibly useful ways to
get involved without coding at all. If you do want to contribute code, however,
it'd be good to have some proficiency with go.

### How is a contribution reviewed and accepted?

- **If you are opening an issue...**

  - Fill out all required sections for your issue type. Issues that are not
    filled out properly will be flagged as `need: more info` and will be closed if not
    updated.
  - _Keep your issue simple, clear, and to-the-point_. Most issues do not
    require many paragraphs of text. In fact, if you write too much, it's
    difficult to understand what you are actually trying to communicate.
    **Consider
    [starting a discussion](https://github.com/livepeer/go-livepeer/discussions/new)
    if you're not clear on something or want feedback from the community.**

- **If you are submitting a pull request...**
  - Write tests to increase code coverage
  - Tag the issue(s) your PR is closing or relating to
  - Make sure your PR is up-to-date with `master` (rebase please 🙏)
  - Wait for a maintainer to review your PR
  - Push additional commits to your PR branch to fix any issues noted in review.
    - Avoid force pushing to the branch which will clobber commit history and makes it more difficult for reviewing incremental changes in the PR
    - Instead use [fixup commits](#commits) that can be squashed prior to merging
  - Wait for a maintainer to merge your PR
    - For a small changesets, the Github "squash and merge" option can be an acceptable
    - For larger changesets, the Github "rebase and merge" option is preferable and a maintainer may request you do a local rebase first to cleanup the branch commit history before merging


### Changelog

Every change (feature, bug fix, etc.) should be made in a PR that includes an update to the `CHANGELOG_PENDING.md` file which tracks the set of changes that will be included in the next release.

Changelog entries should be formatted as follows:

```
- \#xxx Description of the change (@contributor)
```

`xxx` is the PR number (if for whatever reason a PR number cannot be used the issue number is acceptable) and `contributor` is the author of the change. The full link to the PR is not necessary because it will automatically be added when a release is created, but make sure to include the backslash and pound i.e. `\#2313`.

Changelog entries should be classified based on the `livepeer` mode of operation that they pertain to i.e. General, Broadcaster, Orchestrator, Transcoder.

Breaking changes should be documented in the "Breaking changes" section. Any changes that involve pending deprecations (i.e. a flag will be removed in the next release) should be documented in the "Upcoming changes" section.


### When is it appropriate to follow up?

You can expect a response from a maintainer within 7 days. If you haven’t heard
anything by then, feel free to ping the thread.

### How much time is spent on this project?

Currently, there are several teams dedicated to maintaining this project.

### What types of contributions are accepted?

All of the types outlined in [How You Can Help](#how-you-can-help).

### What happens if my suggestion or PR is not accepted?

While it's unlikely, sometimes there's no acceptable way to implement a
suggestion or merge a PR. If that happens, maintainer will still...

- Thank you for your contribution.
- Explain why it doesn’t fit into the scope of the project and offer clear
  suggestions for improvement, if possible.
- Link to relevant documentation, if it exists.
- Close the issue/request.

But do not despair! In many cases, this can still be a great opportunity to
follow-up with an improved suggestion or pull request. Worst case, this repo is
open source, so forking is always an option 😎.
