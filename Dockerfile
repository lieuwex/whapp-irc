FROM golang:1.10.1-alpine3.7 AS builder

RUN apk update && apk add git
RUN go get -u github.com/golang/dep/cmd/dep

RUN mkdir -p /go/src/whapp-irc
WORKDIR /go/src/whapp-irc
COPY . .

RUN dep ensure
RUN GOOS=linux go build -o whapp-irc

#####

FROM alpine:latest AS runner

# Update apk repositories
RUN apk update

# Install chromium
RUN apk --no-cache --allow-untrusted add \
	zlib-dev \
	xvfb \
	wait4ports \
	xorg-server \
	dbus \
	ttf-freefont \
	mesa-dri-swrast \
	grep \
	udev \
	chromium

# Install whapp-irc dependencies and copy whapp-irc
RUN apk --no-cache --allow-untrusted add \
	ca-certificates \
	mailcap
COPY --from=builder /go/src/whapp-irc /bin/

# Remove unneeded stuff
RUN apk del --purge --force \
		linux-headers \
		binutils-gold \
		gnupg \
		zlib-dev \
		libc-utils \
	&& \
	rm -rf /var/lib/apt/lists/* \
		/var/cache/apk/* \
		/usr/share/man \
		/tmp/*

WORKDIR /root

ENTRYPOINT ["/bin/whapp-irc"]
