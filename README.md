# Depreciation
whapp-irc is not supported anymore, and probably never will be anymore by me.
Feel free to fork.

https://git.theta.eu.org/eta/whatsxmpp or https://github.com/eeeeeta/sms-irc or https://github.com/tulir/mautrix-whatsapp/ are all alternatives.

# whapp-irc
_a simple whatsapp web <-> irc gateway_

[take a look at the quick and cool setting up guide](https://github.com/lieuwex/whapp-irc/wiki/Setting-up)

## information
- private chats;
- group chats, with op for admins;
- kicking, inviting, and stuff;
- LIST, WHO (with online/offline state);
- joining chats;
- converts names to irc safe names as much as possible;
- receiving files, hosts it as using a HTTP file server;
- receiving locations, will send a Google Maps link to the location;
- receiving reply messages;
- generating QR code;
- saves login state to disk;
- replay using `whapp-irc/replay` capability;
- IRCv3 `server-time` support;
- no configuration needed;
- probably some stuff I forgot.

## configuration
### irc client
To use whapp-irc optimally you should set the following client capabilities:
- `server-time` (this will show the time when the message was sent in whatsapp
	in your IRC client, instead of when the bridge received it);
- `whapp-irc/replay` (this will replay all the messages the bridge missed, for
	example: when the bridge is turned off. The bridges stores the timestamp of
	the last message for every chat on disk and will send all newer messages to
	the client).

### environment variables
All configuration is done using environment variables.
Quick and simple.
- `HOST`: the IP/domain used to generate the URLs to media files;
- `FILE_SERVER_PORT`: the port used for the file httpserver, if not 80 it will
	be appended to the URLs;
- `IRC_SERVER_PORT`: the port to listen on for IRC connections;
- `LOG_LEVEL`: `normal` (default) or `verbose`, if verbose it will log all
	communication between whapp-irc and the chromium instance;
- `MAP_PROVIDER`: The map provider to use for location messages: can be one of
	`googlemaps` (default) or `openstreetmap`.

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

## local build
if you want to build whapp-irc locally (no guaranty it works with your chromium
version and what not), make sure you have go, dep, and chromium.
Then, clone the repo in your `$GOPATH` and run: `./make.bash`.

## support
`#whapp-irc` on freenode, you can mention lieuwex if nobody responds.
