#!/bin/sh

dep ensure \
	&& cat patches/* | patch -p1 \
	&& go build -ldflags "-X main.commit=$(git rev-list -1 HEAD)" -o whapp-irc

IP=`ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1'`

echo
echo "build successful."
echo "make sure chromium is in your path, and run whapp-irc using:"
printf "\t$ env HOST=$IP whapp-irc\n"
