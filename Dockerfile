# This Dockerfile is consumed by goreleaser, which has already cross-compiled
# the binary for the target platform. It deliberately does not build anything:
# the COPY source is an artifact goreleaser places in the build context.
FROM gcr.io/distroless/static-debian12:nonroot

COPY wherefolk /usr/local/bin/wherefolk

# The document lives on a mounted volume; see ADR-0006. config.DefaultDataDir
# already points here, so WHEREFOLK_DATA only needs setting to override it.
VOLUME ["/var/lib/wherefolk"]

# ADR-0007: the port is configurable via WHEREFOLK_PORT, the interface is not.
# config.DefaultPort is 8080.
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/wherefolk"]
