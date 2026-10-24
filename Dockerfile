# syntax=docker/dockerfile:1
# Stumble Guys' backend (api/matchmaking/localization.stumbleguys.com) for the Switch build.
# Plain HTTP on 23550 behind the TLS terminator; health on 23552.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOWORK=off go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/stumbled ./cmd/stumbled

FROM gcr.io/distroless/static
COPY --from=build /out/stumbled /stumbled
USER nonroot:nonroot
EXPOSE 23550 23552
ENTRYPOINT ["/stumbled", "-data", "/data"]
