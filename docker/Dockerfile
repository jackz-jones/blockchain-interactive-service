FROM golang:1.22.7 as builder
COPY . /chain-interactive-service
RUN cd /chain-interactive-service && make build

# the second stage
FROM ubuntu:24.04
RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN sed -i "s@http://.*archive.ubuntu.com@http://mirrors.huaweicloud.com@g" /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i "s@http://.*security.ubuntu.com@http://mirrors.huaweicloud.com@g" /etc/apt/sources.list.d/ubuntu.sources&& \
    apt-get update && \
    apt-get install -y vim net-tools inetutils-ping telnet tree gcc g++ p7zip-full ca-certificates curl

ENV TZ "Asia/Shanghai"
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y tzdata && \
    echo $TZ > /etc/timezone && \
    ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && \
    dpkg-reconfigure tzdata -f noninteractive

COPY --from=builder /chain-interactive-service/chain-interactive-service /chain-interactive-service/bin/chain-interactive-service
COPY --from=builder /chain-interactive-service/etc/chaininteractive.yaml /chain-interactive-service/etc/chaininteractive.yaml
COPY --from=builder /chain-interactive-service/etc/chainmaker_sdk_config.yml /chain-interactive-service/etc/chainmaker_sdk_config.yml
COPY --from=builder /chain-interactive-service/etc/client.key /chain-interactive-service/etc/client.key
COPY --from=builder /chain-interactive-service/etc/client.pub.key /chain-interactive-service/etc/client.pub.key
COPY --from=builder /chain-interactive-service/etc/nft.json /chain-interactive-service/etc/nft.json
COPY --from=builder /chain-interactive-service/etc/notification.json /chain-interactive-service/etc/notification.json
RUN mkdir -p /chain-interactive-service/logs/
WORKDIR /chain-interactive-service/
CMD ["./bin/chain-interactive-service"]
