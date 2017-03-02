FROM alpine:3.4

COPY . /concept-search-api/

RUN apk --update add bash git bzr go ca-certificates \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/Financial-Times/concept-search-api" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && cp -r concept-search-api/. $GOPATH/src/${REPO_PATH} \
  && rm -r concept-search-api \
  && cd $GOPATH/src/${REPO_PATH} \
  && BUILDINFO_PACKAGE="github.com/Financial-Times/service-status-go/buildinfo." \
  && VERSION="version=$(git describe --tag --always 2> /dev/null)" \
  && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
  && REPOSITORY="repository=$(git config --get remote.origin.url)" \
  && REVISION="revision=$(git rev-parse HEAD)" \
  && BUILDER="builder=$(go version)" \
  && LDFLAGS="-X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
  && echo $LDFLAGS \
  && go get -u github.com/kardianos/govendor \
  && $GOPATH/bin/govendor sync \
  && go get -t ./... \
  && go build -ldflags="${LDFLAGS}" \
  && mv concept-search-api /concept-search-api \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*

CMD [ "/concept-search-api" ]