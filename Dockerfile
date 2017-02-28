FROM alpine:3.4

COPY . /concept-search-api/

RUN apk add --update bash \
  && apk --update add git bzr go ca-certificates \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/Financial-Times/concept-search-api" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && mv concept-search-api/* $GOPATH/src/${REPO_PATH} \
  && rm -r concept-search-api \
  && cd $GOPATH/src/${REPO_PATH} \
  && go get -t ./... \
  && go build \
  && mv concept-search-api /concept-search-api \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*

CMD [ "/concept-search-api" ]