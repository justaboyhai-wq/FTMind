FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/baoan-policy-collector ./cmd/baoan-policy-collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/baoan-policy-collector /baoan-policy-collector
VOLUME ["/data"]
ENTRYPOINT ["/baoan-policy-collector"]
