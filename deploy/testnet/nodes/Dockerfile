FROM alpine:latest
RUN apk add --no-cache ca-certificates bash perl

RUN echo "------ Node ------"

WORKDIR "/node"
COPY ./bin/gev ./
COPY ./start_node.sh ./

RUN mkdir -pv ./log
RUN chmod +x ./start_node.sh

ENTRYPOINT ["./start_node.sh"]
EXPOSE 8545 30303 6060