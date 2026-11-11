FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/funnel-gen3-auth .

FROM alpine:3.22
COPY --from=build /out/funnel-gen3-auth /funnel-gen3-auth
ENTRYPOINT ["/bin/sh"]
