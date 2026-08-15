FROM gcr.io/distroless/static:nonroot

ARG TARGETARCH
COPY dist/movieswipe-linux-${TARGETARCH} /movieswipe
ENTRYPOINT ["/movieswipe"]
