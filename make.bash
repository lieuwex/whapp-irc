#!/usr/bin/env bash

set -euo pipefail

dep ensure
cat patches/* | patch -p1
go build -ldflags "-X main.commit=$(git rev-list -1 HEAD)" -o whapp-irc

{
	case "$OSTYPE" in
		linux*) IP=$(ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p') ;;
		darwin*) IP=$(ipconfig getifaddr en0) ;;
		*) false ;;
	esac
} || IP='<your hostname or IP>'

echo
echo "build successful."
echo "make sure chromium is in your path, and run whapp-irc using:"
printf "\t$ env HOST=$IP whapp-irc\n"
