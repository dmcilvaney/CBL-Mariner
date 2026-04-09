FROM mcr.microsoft.com/azurelinux/base/core:3.0

# Install mock and dependencies needed for azldev component render.
# Go and azldev are bind-mounted from the host via GOPATH.
RUN tdnf -y install \
    ca-certificates \
    git \
    mock \
    mock-rpmautospec \
    python3 \
    shadow-utils \
    sudo \
    && tdnf clean all

ARG UID=1000

RUN useradd -u "${UID}" -G mock -m builduser

USER builduser
WORKDIR /workdir
