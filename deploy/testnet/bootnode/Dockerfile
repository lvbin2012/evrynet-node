FROM alpine:latest
RUN apk add --no-cache ca-certificates

RUN echo "------ Bootnode ------"

WORKDIR "/bootnode"
COPY ./bin/bootnode .

CMD ./bootnode -nodekeyhex "$NODE_HEX_KEY" -verbosity 5 -addr ":30300"

EXPOSE 30300
