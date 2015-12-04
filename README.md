# Installation

Binaries are available in the
[releases page](https://github.com/encryptio/git-annex-remote-b2/releases).

To build from source, [set up a GOPATH](https://golang.org/doc/code.html) and
then run `go get github.com/encryptio/git-annex-remote-b2`.

# Usage

After putting `git-annex-remote-b2` in your `$PATH`, use it like any other
external remote:

    ~/repo $ git annex initremote b2 type=external externaltype=b2 bucket=mydata

B2 credentials may either be given as arguments to `initremote` (
`accountid=XXXX appkey=XXXXXXXXXXXXXXXX`) or as the environment variables
`$B2_APP_KEY` and `$B2_ACCOUNT_ID`. If you pass them as arguments to `initremote`,
the credentials will be stored in the git-annex repository and thus will be
available to all clones of it.
