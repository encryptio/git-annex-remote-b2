#!/bin/bash
set -e

if [ -z "$B2_APP_KEY" ]; then
    echo "Set \$B2_APP_KEY"
    exit 1
fi

if [ -z "$B2_ACCOUNT_ID" ]; then
    echo "Set \$B2_ACCOUNT_ID"
    exit 1
fi

DIR="$(pwd)/integration-test"

if [ -e "$DIR" ]; then
    chmod -R a+w "$DIR"
    rm -rf "$DIR"
fi

mkdir "$DIR"
mkdir "$DIR/bin"

go build -o "$DIR/bin/git-annex-remote-b2"
export PATH="$DIR/bin:$PATH"

pushd "$DIR"
git init
git annex init

BUCKET_NAME="git-annex-test-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)"

git annex initremote noencrypt type=external externaltype=b2 encryption=none bucket="$BUCKET_NAME" prefix=raw
git annex initremote encrypt type=external externaltype=b2 encryption=shared bucket="$BUCKET_NAME" prefix=enc

cp bin/git-annex-remote-b2 somefile
git annex add somefile
git commit -m 'commit'

git annex copy --to noencrypt
git annex fsck --from noencrypt
git annex drop
git annex move --from noencrypt
git annex fsck --from noencrypt

git annex copy --to encrypt
git annex fsck --from encrypt
git annex drop
git annex move --from encrypt
git annex fsck --from encrypt

git annex testremote --fast encrypt
git annex testremote --fast noencrypt

popd
chmod -R a+w "$DIR"
rm -rf "$DIR"

echo "Passed!"
exit 0

