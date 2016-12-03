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
