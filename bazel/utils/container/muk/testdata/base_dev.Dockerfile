FROM us-docker.pkg.dev/enfabrica-container-images/third-party-prod/docker.io/library/ubuntu@sha256:e722c7335fdd0ce77044ab5942cb1fbd2b5f60d1f5416acfcdb0814b2baf7898
COPY . /astore
RUN dpkg --add-architecture i386
RUN apt-add-repository ppa:git-core/ppa
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    apt-transport-https \
    asciidoc \
    automake-1.15 \
    awscli \
    babeltrace \
    bash-completion \
    bc \
    binutils-dev \
    bison \
    build-essential \
    ca-certificates \
    clang \
    clang-format-10 \
    clang-tools \
    cloud-utils \
    cmake \
    cpio \
    cscope \
    csh \
    ctags \
    curl \
    dbus-x11 \
    dialog \
    diffutils \
    dnsutils \
    docker-compose \
    docker.io \
    dwarves \
    emacs \
    ethtool \
    fakeroot \
    fd-find \
    flex \
    g++-10 \
    gcc \
    gcc-10 \
    gdb \
    gdbserver \
    gedit \
    gettext \
    git \
    gnumeric \
    gnupg \
    graphviz \
    grep-dctrl \
    gsfonts-x11 \
    htop \
    iperf \
    iputils-ping \
    jq \
    kate \
    kernel-wedge \
    kmod \
    latexmk \
    less \
    lib32gcc1 \
    lib32stdc++6 \
    lib32z1 \
    libasound2 \
    libaudit-dev \
    libc6-i386 \
    libcap-dev \
    libelf-dev \
    libffi-dev \
    libfontenc1 \
    libgbm-dev \
    libguestfs-tools \
    libiberty-dev \
    libice6 \
    libltdl-dev \
    liblttng-ust-dev \
    libncurses5-dev \
    libnuma-dev \
    libopenmpi-dev \
    libpcap-dev \
    libpci-dev \
    librsvg2-bin \
    libsm6 \
    libsnappy-dev \
    libssl-dev \
    libsystemd-dev \
    libtool-bin \
    libudev-dev \
    libunwind-dev \
    libva-x11-2 \
    libverilog-perl \
    libx11-protocol-perl \
    libx11-xcb1 \
    libxaw7 \
    libxft2 \
    libxi6 \
    libxi6:i386 \
    libxmu6 \
    libxpm4 \
    libxrender1 \
    libxrender1:i386 \
    libxt6 \
    libxv1 \
    libzstd-dev \
    linux-base \
    linux-tools-common \
    llvm \
    lsb-core \
    lttng-tools \
    make \
    meld \
    mosh \
    ncurses-dev \
    neovim \
    net-tools \
    netcat \
    netperf \
    ninja-build \
    openjdk-11-jdk \
    openjdk-11-jre \
    openmpi-bin \
    openmpi-common \
    openssh-client \
    pigz \
    pkg-config \
    pv \
    python-dev \
    python3-dev \
    python3-distutils \
    python3-docutils \
    python3-pyelftools \
    python3.8-venv \
    qpdfview \
    ripgrep \
    rsync \
    screen \
    shellcheck \
    software-properties-common \
    strace \
    stressapptest \
    sudo \
    sysstat \
    tcpdump \
    tcsh \
    terminator \
    tex-gyre \
    texlive-base \
    texlive-bibtex-extra \
    texlive-binaries \
    texlive-extra-utils \
    texlive-font-utils \
    texlive-fonts-recommended \
    texlive-latex-base \
    texlive-latex-extra \
    texlive-latex-recommended \
    texlive-pictures \
    texlive-plain-generic \
    texlive-science \
    tkdiff \
    tmux \
    tree \
    vim \
    vim-gtk \
    vim-youcompleteme \
    wget \
    x11-apps \
    x11-utils \
    x11-xserver-utils \
    xbitmaps \
    xfce4 \
    xmlto \
    xterm \
    xz-utils \
    zip
RUN curl -fsSL 'https://packages.cloud.google.com/apt/doc/apt-key.gpg' | apt-key --keyring '/usr/share/keyrings/cloud.google.gpg' add -
RUN echo 'deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main' | tee -a /etc/apt/sources.list.d/google_cloud_sdk.list
RUN curl -fsSL 'https://apt.releases.hashicorp.com/gpg' | apt-key --keyring '/usr/share/keyrings/hashicorp.gpg' add -
RUN echo 'deb [signed-by=/usr/share/keyrings/hashicorp.gpg arch=amd64] https://apt.releases.hashicorp.com focal main' | tee -a /etc/apt/sources.list.d/hashicorp.list
RUN wget -O /usr/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/download/v1.9.0/bazelisk-linux-amd64
RUN chmod 0777 /usr/bin/bazelisk
RUN ln -sf /usr/bin/bazelisk /usr/bin/bazel
RUN sh /astore/cuda_12_linux.run --silent --installpath=/usr/local/cuda-12.1 --toolkit --no-man-page --no-opengl-libs --no-drm
RUN rm -rf /usr/local/cuda/extras /usr/local/cuda/libnvvp /usr/local/cuda/nsight*
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    clangd \
    google-cloud-sdk \
    google-cloud-sdk-app-engine-python \
    google-cloud-sdk-gke-gcloud-auth-plugin \
    kubectl \
    packer \
    unzip \
    /astore/libprotobuf7_i386.deb \
    /astore/chrome-remote-desktop_current_amd64.deb \
    /astore/gh.deb \
    /astore/cudnn-local-repo.deb
RUN cp /var/cudnn-local-repo-ubuntu2004-8.8.1.3/cudnn-local-*-keyring.gpg /usr/share/keyrings/
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \
    libcudnn8-dev
RUN locale-gen en_US.UTF-8
RUN chmod -x /etc/update-motd.d/*
RUN echo 'dash dash/sh boolean false' | debconf-set-selections
RUN dpkg-reconfigure dash
RUN mkdir -p /lib/modules
RUN apt-get autoclean -y
RUN rm -rf /var/cache/apt/* /var/lib/apt/lists/* "${HOME}/.cache" /tmp/*
RUN rm -rf /astore
