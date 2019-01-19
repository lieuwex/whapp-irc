FROM golang:1.11.4-alpine3.8 AS builder

# Install dep
RUN apk update && apk add git patch
RUN go get -u github.com/golang/dep/cmd/dep

# Install dependencies
COPY Gopkg.lock Gopkg.toml /go/src/whapp-irc/
WORKDIR /go/src/whapp-irc/
RUN dep ensure -vendor-only

# Apply chromedp patch
COPY chromedp.patch .
WORKDIR /go/src/whapp-irc/vendor/github.com/chromedp/chromedp/
RUN cat chromedp.patch | patch -p1

# Build whapp-irc
WORKDIR /go/src/whapp-irc/
COPY . .
RUN go build -ldflags "-X main.commit=$(git rev-list -1 HEAD)" -o /bin/whapp-irc

#####

FROM alpine:3.7 AS runner

# Update apk repositories
RUN apk update \
	&& apk --no-cache add \
		# Install chromium
		chromium \
		# Install whapp-irc dependencies
		ca-certificates \
		mailcap \
	&& apk del --purge --force \
		linux-headers \
		binutils-gold \
		gnupg \
		zlib-dev \
		libc-utils \
	&& rm -rf /var/lib/apt/lists/* \
		/var/cache/apk/* \
		/usr/share/man \
		/tmp/*

# Copy whapp-irc
COPY --from=builder /bin/whapp-irc /bin/whapp-irc

WORKDIR /root
ENTRYPOINT ["/bin/whapp-irc"]
