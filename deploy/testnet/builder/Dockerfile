# Build Geth in a stock Go builder container
FROM golang:1.12-alpine
RUN echo "------ Builder ------"
RUN apk add --no-cache make gcc musl-dev linux-headers git

# Create the user and group files that will be used in the running
# container to run the process as an unprivileged user.
RUN mkdir /user && \
 echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
 echo 'nobody:x:65534:' > /user/group

# Copy the predefined token file into the location that git depends on
COPY ./token /root/.netrc
RUN chmod 600 /root/.netrc

RUN cat /root/.netrc

WORKDIR "/evrynet"
COPY ./project ./project
RUN cd ./project && \
    go build ./cmd/gev && \
    go build ./cmd/bootnode && \
    cd .. && \
    cp ./project/gev ./ && \
    cp ./project/bootnode ./ && \
    rm -rf ./project