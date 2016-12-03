Installation
============

Binaries are available in the [releases page](https://github.com/encryptio/git-annex-remote-b2/releases). You want to add the binary to your `$PATH`, either by creating a new directory for it (ideally inside your home directory, such as `~/bin`), or by putting the binary in a directory that is already in your `$PATH` (such as `/usr/local/bin`.)

To build from source, [set up a GOPATH](https://golang.org/doc/code.html) and then run `go get github.com/encryptio/git-annex-remote-b2`.

Usage
=====

After putting `git-annex-remote-b2` in your `$PATH`, use it like any other external remote:

```
~/repo $ git annex initremote b2 type=external externaltype=b2 bucket=mydata
```

B2 credentials may either be given as arguments to `initremote` ( `accountid=XXXX appkey=XXXXXXXXXXXXXXXX`) or as the environment variables `$B2_APP_KEY` and `$B2_ACCOUNT_ID`. If you pass them as arguments to `initremote`, the credentials will be stored in the git-annex repository and thus will be available to all clones of it.

Optionally, you may pass `prefix=something` to have `git-annex-remote-b2` prepend `something/` to the keys it stores in B2.

Improving the financial cost of this remote
-------------------------------------------

By default, all remotes are `semitrusted` in git-annex. This means that the remote should be checked to see if it actually has data when doing an operation that assumes that the data is safe if they have it and unsafe if not (for example, a *local* `git annex drop`.) If you tell git-annex that this remote won't lose data randomly by setting this remote's trust level higher, then those `checkpresentkey` calls (which turn into `ListFileNames` calls on B2) should go away.

This is particularly important if you're under the free trial limits of B2.

```
~/repo $ git annex trust b2
```

Secondly, `git-annex` will assume all non-local remotes have the same cost, and won't prefer one over the other by default. If you have a remote that doesn't cost as much as talking to B2, you should set the B2 remote's cost very high so that `git-annex` will prefer talking to the cheap remote rather than B2 when possible. (The default values are 100 for local remotes, and 200 for non-local remotes.)

```
~/repo $ git config remote.b2.annex-cost 1000
```

Note that setting the `annex-cost` like this is a repo-local operation only; it does not apply to other clones of the repo you might have.
