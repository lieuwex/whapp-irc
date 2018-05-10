# whapp-irc
_a simple whatsapp web <-> irc gateway_

## information
- private chats;
- group chats, with op for admins;
- kicking, inviting, and stuff;
- LIST, WHO (with online/offline state);
- joining chats;
- converts names to irc safe names as much as possible;
- receiving files, hosts it as using a HTTP file server;
- receiving reply messages;
- generating QR code;
- saves login state to disk;
- replay using `whapp-irc/replay` capability;
- IRCv3 `server-time` support;
- no configuration needed;
- probably some stuff I forgot.

## docker
It's recommend to use the docker image.
It's also the only supported version, since this way we have a consistent,
predictable and reproducible version.

To run:
```
docker run -d \
	--name whapp-irc \
	-v PATH_TO_DIR_FOR_DATA_HERE:/root \
	-p 6667:6060 \
	-p 3000:3000 \
	-e "HOST=IP_ADDRESS_OR_DOMAIN_HERE" \
	lieuwex/whapp-irc
```

## installation
make sure you have go and dep, then clone the repo in your `$GOPATH` and:
```shell
dep ensure
go build
./whapp-irc
```
