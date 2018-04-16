# whapp-irc
_a simple whatsapp web <-> irc gateway_

## information
- private chats;
- group chats, with op for admins;
- LIST;
- joining chats;
- converts names to irc safe names as much as possible;
- receiving files, hosts it as using a HTTP file server;
- receiving reply messages;
- generating QR code;
- saves login state to disk;
- probably some stuff I forgot.

## installation
make sure you have python3, then clone the repo in your `$GOPATH` and:
```shell
pip3 install -r requirements.txt
go build
./whapp-irc
```

## docker
soon™
