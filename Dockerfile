# This Dockerfile is consumed by goreleaser, which has already cross-compiled
# the binary for every target platform. It deliberately does not build the Go
# binary: the COPY source is an artifact goreleaser places in the build context.
#
# dockers_v2 lays that context out per platform — linux/amd64/wherefolk,
# linux/arm64/wherefolk — so TARGETPLATFORM, which buildx sets per architecture,
# is what lets one Dockerfile serve both.

# The typst stage fetches the renderer. It is separate so that curl, tar and the
# tarball itself never reach the final image: only the unpacked binary is copied
# forward.
#
# Typst is installed from its official release tarball at a pinned version and
# checksum (§2.1). Pinning the version inside the image is the point of shipping
# a container at all — a host upgrade can then never silently reflow the
# Directory. The checksum is what makes the pin meaningful: without it, a
# re-tagged upstream release would be installed without complaint.
FROM debian:12-slim AS typst

ARG TARGETARCH
ARG TYPST_VERSION=0.14.2

# Upstream publishes only musl builds for amd64 and arm64, and they are
# static-pie linked — no libc dependency, so they run on Debian unchanged.
# (§2.1 warns that Typst ships glibc binaries and musl is a risk; that was true
# of earlier releases and is not true of 0.14.2. See ADR-0004.)
ARG TYPST_SHA256_amd64=a6044cbad2a954deb921167e257e120ac0a16b20339ec01121194ff9d394996d
ARG TYPST_SHA256_arm64=491b101aa40a3a7ea82a3f8a6232cabb4e6a7e233810082e5ac812d43fdcd47a

# hadolint ignore=DL3008
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl xz-utils; \
    rm -rf /var/lib/apt/lists/*; \
    case "${TARGETARCH}" in \
      amd64) arch=x86_64;  sha="${TYPST_SHA256_amd64}" ;; \
      arm64) arch=aarch64; sha="${TYPST_SHA256_arm64}" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    file="typst-${arch}-unknown-linux-musl.tar.xz"; \
    curl -fsSLO "https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/${file}"; \
    echo "${sha}  ${file}" | sha256sum -c -; \
    tar -xJf "${file}" --strip-components=1 -C /usr/local/bin "typst-${arch}-unknown-linux-musl/typst"; \
    rm -f "${file}"; \
    /usr/local/bin/typst --version


FROM debian:12-slim

ARG TARGETPLATFORM

# ca-certificates only: the binary binds loopback and talks to nothing outbound
# except item 14's Healthchecks.io ping, which needs a trust store.
# hadolint ignore=DL3008
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates; \
    rm -rf /var/lib/apt/lists/*; \
    groupadd --system --gid 65532 nonroot; \
    useradd --system --uid 65532 --gid 65532 --no-create-home nonroot

COPY --from=typst /usr/local/bin/typst /usr/local/bin/typst

COPY $TARGETPLATFORM/wherefolk /usr/local/bin/wherefolk

# The Typst layout ships in the image rather than on the volume, because it is
# versioned with the code that generates markup for it: a template expecting
# arguments the binary no longer emits would fail at export time. ADR-0004's
# "edit the template and restart" affordance is preserved by mounting over this
# path or pointing WHEREFOLK_TEMPLATE elsewhere.
#
# config.DefaultTemplateDir is /var/lib/wherefolk/template, which sits under the
# volume below — so it is set explicitly here to a path the volume cannot shadow.
COPY template/ /usr/local/share/wherefolk/template/
ENV WHEREFOLK_TEMPLATE=/usr/local/share/wherefolk/template

# The document lives on a mounted volume; see ADR-0006. config.DefaultDataDir
# already points here, so WHEREFOLK_DATA only needs setting to override it.
VOLUME ["/var/lib/wherefolk"]

# ADR-0007: the port is configurable via WHEREFOLK_PORT, the interface is not.
# config.DefaultPort is 8080.
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/wherefolk"]
