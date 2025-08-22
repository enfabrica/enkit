FROM us-docker.pkg.dev/enfabrica-container-images/third-party-prod/docker.io/library/ubuntu@sha256:e722c7335fdd0ce77044ab5942cb1fbd2b5f60d1f5416acfcdb0814b2baf7898
# COPY . /astore
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    apt-utils \
    curl \
    gnupg2 \
    software-properties-common \
    apt-transport-https

RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    software-properties-common
RUN curl -fsSL 'https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x23F3D4EA75716059' | apt-key --keyring '/usr/share/keyrings/github_cli.gpg' add -
RUN echo 'deb [signed-by=/usr/share/keyrings/github_cli.gpg] https://cli.github.com/packages focal main' | tee -a /etc/apt/sources.list.d/github_cli.list
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    apt-utils \
    bc \
    ca-certificates \
    cpio \
    curl \
    git \
    gh \
    kmod \
    lsb-release \
    pkg-config \
    python3-dev \
    rsync \
    xmlto \
    build-essential \
    kernel-wedge \
    grep-dctrl \
    flex \
    bison \
    libudev-dev \
    libusb-dev \
    fakeroot \
    libpci-dev \
    libssl-dev \
    libelf-dev \
    libiberty-dev \
    libcap-dev \
    python3-docutils \
    asciidoc \
    jq \
    zstd

# Add arm64 sources that actually exist
RUN echo 'deb [arch=arm64] http://ports.ubuntu.com/ubuntu-ports/ focal main restricted universe multiverse' >> /etc/apt/sources.list
RUN echo 'deb [arch=arm64] http://ports.ubuntu.com/ubuntu-ports/ focal-updates main restricted universe multiverse' >> /etc/apt/sources.list
RUN echo 'deb [arch=arm64] http://ports.ubuntu.com/ubuntu-ports/ focal-security main restricted universe multiverse' >> /etc/apt/sources.list

RUN dpkg --add-architecture arm64
RUN dpkg --print-architecture
RUN dpkg --print-foreign-architectures

RUN apt update || true

RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get install -y \
    libpci-dev:arm64

RUN echo 'deb [ snapshot=yes] http://archive.ubuntu.com/ubuntu/ oracular main restricted' | tee -a /etc/apt/sources.list.d/oracular_snapshot.list
RUN echo 'deb [ snapshot=yes] http://archive.ubuntu.com/ubuntu/ oracular-updates main restricted' | tee -a /etc/apt/sources.list.d/oracular_updates_snapshot.list
RUN echo 'deb [ snapshot=yes] http://security.ubuntu.com/ubuntu oracular-security main restricted' | tee -a /etc/apt/sources.list.d/oracular_security_snapshot.list

RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    /astore/libtraceevent1_1.8.2-1ubuntu3_amd64.deb \
    libtraceevent-dev=1:1.8.2-1ubuntu3
RUN tar -xvf /astore/arm-gnu-toolchain-13.3.rel1-x86_64-aarch64-none-linux-gnu.tar.xz -C /usr --strip-components 1
RUN curl -Lo /usr/local/bin/enkit https://astore.corp.enfabrica.net/d/tools/enkit?a=amd64-linux && chmod 0755 /usr/local/bin/enkit
RUN apt-get autoclean -y
RUN rm -rf /var/cache/apt/* /var/lib/apt/lists/* "${HOME}/.cache" /tmp/*
LABEL STABLE_GIT_MASTER_SHA=97e014b8d7d3602d152a29db3ff02591748c3129
LABEL GIT_BRANCH=SF-1573-enable-deb-artifacts-for-arm-64-kernel-build
LABEL BUILD_USER=roman
LABEL BUILD_TIME=20250822-210003.023231454
LABEL CLEAN_BUILD=False
LABEL OFFICIAL_BUILD=False
RUN rm -rf /astore
