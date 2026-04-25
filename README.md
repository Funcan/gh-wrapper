
A golang wrapper around Github's `gh` CLI.

## The Problem

Assume I've got a work github account, and a personal github account, and some
repos on my machine I want to work with on one account, and some the other.

## The solution

This tool acts as a wrapper around `gh` - if configured in the user's shell
via `alias gh=gh-wrapper` then it acts as an automatic switch of `gh` user, by
calling `gh auth user switch --user USERNAME` just before calling the desired
`gh` command, and resetting it immediately afterwards (including in the case
of failed or aborted commands)

To figure out what USERNAME to use it checks:
.git/config `gh-wrapper/user` key

If we aren't in a github repo, or the above isn't set, then it instead reads
`~/.gh-wrapper.conf` which contains a set of lines referring to either local
directorys or github servers/orgs/repositories. The first match wins so put
the most specific match first. An example `.gh_wrapper.conf`:

```
directory ~/personal: my-personal-user
github my-work-org1: my-work-account
github mywork-org2: my-work-account
github someorg/repo-foo: my-work-account
github :my-personal-account
```
