// NOTE: if you change this file, be sure to also update the expected data file
// in //infra/dev_container/muk/testdata/base_dev.Dockerfile.
{
  // Returns the packages used in the dev base image
  DevBasePackages:: function() $._Dedup(
    $._ForBaseOs('ubuntu', $.infra_tools + $.sw_test_tools + $.misc_packages)
  ),

  UbuntuHwDevPackages:: function() $._Dedup(
    $._ForBaseOs(
      'ubuntu',
      $.arm_pkgs +
      $.verilator_pkgs +
      $.vcs_pkgs +
      $.verdi_pkgs +
      $.vc_static_pkgs +
      $.innovus_pkgs +
      $.icv_workbench_pkgs +
      $.cadence_palladium_pkgs +
      $.tsmc_pkgs +
      $.iscape_gui_pkgs +
      $.vmanager_pkgs
    )
  ),
  
  CentosBasePackages:: function() $._Dedup(
    $._ForBaseOs(
      'centos',
      $.default_text_editors +
      $.pd_team_custom_text_editors +
      $.pd_team_data_transfer_pkgs +
      $.pd_team_parsing_pkgs + 
      $.pd_team_shell_pkgs + 
      $.pd_team_tech_doc_pkgs +
      $.pd_team_file_comparison_pkgs +
      $.pd_team_file_filtering_pkgs +
      $.permissions_pkgs + 
      $.bazel_workflow_pkgs + 
      $.git_workflow_pkgs + 
      $.gee_pkgs + 
      $.infra_tools +
      $.synopsys_dft_shell_pkgs + 
      $.synopsys_testmax_atpg_pkgs +
      $.synopsys_icv_pkgs + 
      $.ausdia_timevision_pkgs +
      $.ansys_edt_pkgs +
      $.cadence_common_pkgs + 
      $.quantus_pkgs +
      $.keysight_pathwave_ads_pkgs +
      $.python3_build_pkgs +
      $.git2_build_pkgs +
      $.cjfs_pkgs
    )
  ),

  // Each package is a dict of the respective package name for the given OS. By
  // not filling in an OS-specific package name, that package should be omitted
  // for any list for that OS.
  _Package:: function(ubuntu_name=null, centos_name=null) {
    ubuntu: ubuntu_name,
    centos: centos_name,
  },

  // Produce a list of package names for a specific OS.
  _ForBaseOs:: function(os_name, packages) [
    p[os_name]
    for p in packages
    if p[os_name] != null
  ],

  // Deduplicate entries in a string list
  _Dedup:: function(packages) std.uniq(std.sort((packages))),

  // Tools required by infra-team for interactive debugging
  infra_tools:: [
    $._Package(ubuntu_name='fd-find'),
    $._Package(ubuntu_name='dnsutils'),
    $._Package(ubuntu_name='net-tools'),
    $._Package(ubuntu_name='ripgrep'),
    // BUG(INFRA-2771): Allows infra-team to easily induce OoM conditions to test
    // protections from machines going AWOL
    $._Package(ubuntu_name='stressapptest'),
    $._Package(centos_name='dnf'),
    $._Package(centos_name='strace'),
    $._Package(centos_name='net-tools'),
    $._Package(centos_name='sysstat'),
    $._Package(centos_name='tcpdump'),
    $._Package(centos_name='netcat'),
    $._Package(centos_name='sudo'),
    $._Package(centos_name='ripgrep'),
    $._Package(centos_name='tree'),
    $._Package(centos_name='wget'),
  ],

  sw_test_tools:: [
    $._Package(ubuntu_name='ethtool'),
  ],

  // TODO(bbhuynh): Categorize these further according to purpose (who asked for
  // them/what higher-level function requires them)
  misc_packages:: [
    $._Package(ubuntu_name='apt-transport-https'),
    $._Package(ubuntu_name='asciidoc'),
    $._Package(ubuntu_name='automake-1.15'),
    $._Package(ubuntu_name='awscli'),
    $._Package(ubuntu_name='babeltrace'),
    $._Package(ubuntu_name='bash-completion'),
    $._Package(ubuntu_name='bc'),
    $._Package(ubuntu_name='binutils-dev'),
    $._Package(ubuntu_name='bison'),
    $._Package(ubuntu_name='build-essential'),
    $._Package(ubuntu_name='ca-certificates'),
    $._Package(ubuntu_name='clang-format-10'),
    $._Package(ubuntu_name='clang'),
    $._Package(ubuntu_name='clang-tools'),
    $._Package(ubuntu_name='cloud-utils'),
    $._Package(ubuntu_name='cmake'),
    $._Package(ubuntu_name='cpio'),
    $._Package(ubuntu_name='cscope'),
    $._Package(ubuntu_name='csh'),
    $._Package(ubuntu_name='ctags'),
    $._Package(ubuntu_name='curl'),
    $._Package(ubuntu_name='dbus-x11'),
    $._Package(ubuntu_name='dialog'),
    $._Package(ubuntu_name='diffutils'),
    $._Package(ubuntu_name='docker.io'),
    $._Package(ubuntu_name='docker-compose'),
    $._Package(ubuntu_name='dwarves'),
    $._Package(ubuntu_name='emacs'),
    $._Package(ubuntu_name='fakeroot'),
    $._Package(ubuntu_name='flex'),
    $._Package(ubuntu_name='g++-10'),
    $._Package(ubuntu_name='gcc'),
    $._Package(ubuntu_name='gcc-10'),
    $._Package(ubuntu_name='gdb'),
    $._Package(ubuntu_name='gdbserver'),
    $._Package(ubuntu_name='gedit'),
    $._Package(ubuntu_name='gettext'),
    $._Package(ubuntu_name='git'),
    $._Package(ubuntu_name='gnupg'),
    $._Package(ubuntu_name='gnumeric'),
    $._Package(ubuntu_name='graphviz'),
    $._Package(ubuntu_name='grep-dctrl'),
    $._Package(ubuntu_name='gsfonts-x11'),
    $._Package(ubuntu_name='htop'),
    $._Package(ubuntu_name='iperf'),
    $._Package(ubuntu_name='iputils-ping'),
    $._Package(ubuntu_name='jq'),
    $._Package(ubuntu_name='kate'),
    $._Package(ubuntu_name='kernel-wedge'),
    $._Package(ubuntu_name='kmod'),
    $._Package(ubuntu_name='latexmk'),
    $._Package(ubuntu_name='less'),
    $._Package(ubuntu_name='lib32gcc1'),
    $._Package(ubuntu_name='lib32stdc++6'),
    $._Package(ubuntu_name='lib32z1'),
    $._Package(ubuntu_name='libasound2'),
    $._Package(ubuntu_name='libaudit-dev'),
    $._Package(ubuntu_name='libc6-i386'),
    $._Package(ubuntu_name='libcap-dev'),
    $._Package(ubuntu_name='libelf-dev'),
    $._Package(ubuntu_name='libffi-dev'),
    $._Package(ubuntu_name='libfontenc1'),
    $._Package(ubuntu_name='libgbm-dev'),
    $._Package(ubuntu_name='libguestfs-tools'),
    $._Package(ubuntu_name='libiberty-dev'),
    $._Package(ubuntu_name='libice6'),
    $._Package(ubuntu_name='libltdl-dev'),
    $._Package(ubuntu_name='liblttng-ust-dev'),
    $._Package(ubuntu_name='libncurses5-dev'),
    $._Package(ubuntu_name='libnuma-dev'),
    $._Package(ubuntu_name='libopenmpi-dev'),
    $._Package(ubuntu_name='libpcap-dev'),
    $._Package(ubuntu_name='libpci-dev'),
    $._Package(ubuntu_name='librsvg2-bin'),
    $._Package(ubuntu_name='libsm6'),
    $._Package(ubuntu_name='libsnappy-dev'),
    $._Package(ubuntu_name='libssl-dev'),
    $._Package(ubuntu_name='libsystemd-dev'),
    $._Package(ubuntu_name='libtool-bin'),
    $._Package(ubuntu_name='libudev-dev'),
    $._Package(ubuntu_name='libunwind-dev'),
    $._Package(ubuntu_name='libva-x11-2'),
    $._Package(ubuntu_name='libverilog-perl'),
    $._Package(ubuntu_name='libx11-protocol-perl'),
    $._Package(ubuntu_name='libx11-xcb1'),
    $._Package(ubuntu_name='libxaw7'),
    $._Package(ubuntu_name='libxft2'),
    $._Package(ubuntu_name='libxi6'),
    $._Package(ubuntu_name='libxi6:i386'),
    $._Package(ubuntu_name='libxmu6'),
    $._Package(ubuntu_name='libxpm4'),
    $._Package(ubuntu_name='libxrender1'),
    $._Package(ubuntu_name='libxrender1:i386'),
    $._Package(ubuntu_name='libxt6'),
    $._Package(ubuntu_name='libxv1'),
    $._Package(ubuntu_name='libzstd-dev'),
    $._Package(ubuntu_name='linux-base'),
    $._Package(ubuntu_name='linux-tools-common'),
    $._Package(ubuntu_name='llvm'),
    $._Package(ubuntu_name='lsb-core'),
    $._Package(ubuntu_name='lttng-tools'),
    $._Package(ubuntu_name='make'),
    $._Package(ubuntu_name='meld'),
    $._Package(ubuntu_name='mosh'),
    $._Package(ubuntu_name='netcat'),
    $._Package(ubuntu_name='ncurses-dev'),
    $._Package(ubuntu_name='neovim'),
    $._Package(ubuntu_name='netperf'),
    $._Package(ubuntu_name='ninja-build'),
    $._Package(ubuntu_name='openjdk-11-jdk'),
    $._Package(ubuntu_name='openjdk-11-jre'),
    $._Package(ubuntu_name='openmpi-bin'),
    $._Package(ubuntu_name='openmpi-common'),
    $._Package(ubuntu_name='openssh-client'),
    $._Package(ubuntu_name='pigz'),
    $._Package(ubuntu_name='pkg-config'),
    $._Package(ubuntu_name='pv'),
    $._Package(ubuntu_name='python-dev'),
    $._Package(ubuntu_name='python3-dev'),
    $._Package(ubuntu_name='python3-distutils'),
    $._Package(ubuntu_name='python3-docutils'),
    $._Package(ubuntu_name='python3-pyelftools'),
    $._Package(ubuntu_name='python3.8-venv'),
    $._Package(ubuntu_name='qpdfview'),
    $._Package(ubuntu_name='ripgrep'),
    $._Package(ubuntu_name='rsync'),
    $._Package(ubuntu_name='screen'),
    $._Package(ubuntu_name='shellcheck'),
    $._Package(ubuntu_name='software-properties-common'),
    $._Package(ubuntu_name='strace'),
    $._Package(ubuntu_name='sudo'),
    $._Package(ubuntu_name='sysstat'),
    $._Package(ubuntu_name='tcpdump'),
    $._Package(ubuntu_name='tcsh'),
    $._Package(ubuntu_name='terminator'),
    $._Package(ubuntu_name='tex-gyre'),
    $._Package(ubuntu_name='texlive-base'),
    $._Package(ubuntu_name='texlive-bibtex-extra'),
    $._Package(ubuntu_name='texlive-binaries'),
    $._Package(ubuntu_name='texlive-extra-utils'),
    $._Package(ubuntu_name='texlive-font-utils'),
    $._Package(ubuntu_name='texlive-fonts-recommended'),
    $._Package(ubuntu_name='texlive-latex-base'),
    $._Package(ubuntu_name='texlive-latex-extra'),
    $._Package(ubuntu_name='texlive-latex-recommended'),
    $._Package(ubuntu_name='texlive-pictures'),
    $._Package(ubuntu_name='texlive-plain-generic'),
    $._Package(ubuntu_name='texlive-science'),
    $._Package(ubuntu_name='tmux'),
    $._Package(ubuntu_name='tree'),
    $._Package(ubuntu_name='vim'),
    $._Package(ubuntu_name='vim-gtk'),
    $._Package(ubuntu_name='vim-youcompleteme'),
    $._Package(ubuntu_name='wget'),
    $._Package(ubuntu_name='x11-apps'),
    $._Package(ubuntu_name='x11-apps'),
    $._Package(ubuntu_name='x11-utils'),
    $._Package(ubuntu_name='x11-xserver-utils'),
    $._Package(ubuntu_name='xbitmaps'),
    $._Package(ubuntu_name='xmlto'),
    $._Package(ubuntu_name='xterm'),
    $._Package(ubuntu_name='xz-utils'),
    $._Package(ubuntu_name='xfce4'),
    $._Package(ubuntu_name='zip'),
    $._Package(ubuntu_name='tkdiff'),
  ],

  arm_pkgs:: [
    $._Package(ubuntu_name='libxml2'),
    $._Package(ubuntu_name='libxrandr-dev'),
    $._Package(ubuntu_name='libxrandr-dev:i386'),
    $._Package(ubuntu_name='libxcursor-dev'),
    $._Package(ubuntu_name='libxcursor-dev:i386'),
    $._Package(ubuntu_name='libsm-dev'),
    $._Package(ubuntu_name='libice-dev'),
    $._Package(ubuntu_name='libstdc++5'),
    $._Package(ubuntu_name='libstdc++-5-dev'),
    $._Package(ubuntu_name='libstdc++5:i386'),
    $._Package(ubuntu_name='tcsh'),
    $._Package(ubuntu_name='zlib1g-dev'),
  ],

  verilator_pkgs:: [
    $._Package(ubuntu_name='tcl-dev'),
    $._Package(ubuntu_name='tk-dev'),
    $._Package(ubuntu_name='gperf'),
    $._Package(ubuntu_name='libgtk2.0-dev'),
    $._Package(ubuntu_name='gdb'),
    $._Package(ubuntu_name='liblzma-dev'),
  ],

  vcs_pkgs:: [
    $._Package(ubuntu_name='libfreetype6'),
    $._Package(ubuntu_name='libjpeg62'),
    $._Package(ubuntu_name='libmng2'),
    $._Package(ubuntu_name='libncurses5'),
    $._Package(ubuntu_name='libpng16-16'),
    $._Package(ubuntu_name='libssl1.1'),
    $._Package(ubuntu_name='libtiff5'),
    $._Package(ubuntu_name='libxft2'),
    $._Package(ubuntu_name='libxi6'),
    $._Package(ubuntu_name='libxss1'),
    $._Package(ubuntu_name='libelf1:i386'),
    $._Package(ubuntu_name='libc6:i386'),
    $._Package(ubuntu_name='libc6-dev:i386'),
    $._Package(ubuntu_name='g++'),
    $._Package(ubuntu_name='libsm6:i386'),
    $._Package(ubuntu_name='libx11-6:i386'),
    $._Package(ubuntu_name='libxft2:i386'),
    $._Package(ubuntu_name='libxmu6:i386'),
    $._Package(ubuntu_name='libxss1:i386'),
    $._Package(ubuntu_name='libxt6:i386'),
    $._Package(ubuntu_name='libjpeg62-dev:i386'),
    $._Package(ubuntu_name='libncurses5:i386'),
    $._Package(ubuntu_name='libncursesw5:i386'),
    $._Package(ubuntu_name='libtiff-tools'),
    $._Package(ubuntu_name='zlib1g-dev:i386'),
    $._Package(ubuntu_name='g++-multilib'),
    $._Package(ubuntu_name='g++-7-multilib'),
    $._Package(ubuntu_name='libstdc++6-7-dbg'),
    $._Package(ubuntu_name='libstdc++-7-doc'),
    $._Package(ubuntu_name='dc'),
    $._Package(ubuntu_name='g++-multilib'),
  ],

  verdi_pkgs:: [
    $._Package(ubuntu_name='libc6'),
    $._Package(ubuntu_name='gdb'),
    $._Package(ubuntu_name='dc'),
    $._Package(ubuntu_name='libncurses5-dev'),
    $._Package(ubuntu_name='g++'),
    $._Package(ubuntu_name='pstack'),
    $._Package(ubuntu_name='gcc-multilib'),
    $._Package(ubuntu_name='g++-multilib'),
    $._Package(ubuntu_name='libc6-dbg'),
    $._Package(ubuntu_name='libexpat-dev'),
    $._Package(ubuntu_name='libxpm4'),
    $._Package(ubuntu_name='libmng2'),
    $._Package(ubuntu_name='libxft2'),
    $._Package(ubuntu_name='libxmu6'),
    $._Package(ubuntu_name='libjpeg62-dev'),
    $._Package(ubuntu_name='gnome-core'),
    $._Package(ubuntu_name='libxml2'),
    $._Package(ubuntu_name='libxft-dev'),
    $._Package(ubuntu_name='libsm6'),
    $._Package(ubuntu_name='libpng16-16'),
    $._Package(ubuntu_name='libxi6'),
  ],

  vc_static_pkgs:: [
    $._Package(ubuntu_name='libc6'),
    $._Package(ubuntu_name='gdb'),
    $._Package(ubuntu_name='dc'),
    $._Package(ubuntu_name='libncurses5-dev'),
    $._Package(ubuntu_name='g++'),
    $._Package(ubuntu_name='pstack'),
    $._Package(ubuntu_name='gcc-multilib'),
    $._Package(ubuntu_name='g++-multilib'),
    $._Package(ubuntu_name='libc6-dbg'),
    $._Package(ubuntu_name='libexpat-dev'),
    $._Package(ubuntu_name='libxpm4'),
    $._Package(ubuntu_name='libmng2'),
    $._Package(ubuntu_name='libxft2'),
    $._Package(ubuntu_name='libxmu6'),
    $._Package(ubuntu_name='libjpeg62-dev'),
    $._Package(ubuntu_name='libxml2'),
    $._Package(ubuntu_name='libxft-dev'),
    $._Package(ubuntu_name='libsm6'),
    $._Package(ubuntu_name='libpng16-16'),
    $._Package(ubuntu_name='libfontconfig1'),
    $._Package(ubuntu_name='libbz2-1.0'),
    $._Package(ubuntu_name='libxrandr2'),
    $._Package(ubuntu_name='libxi6'),
  ],

  innovus_pkgs:: [
    $._Package(ubuntu_name='libxm4'),
    $._Package(ubuntu_name='libglu1-mesa'),
    $._Package(ubuntu_name='libgfortran3'),
  ],

  icv_workbench_pkgs:: [
    $._Package(ubuntu_name='python3-pyqt5'),
    $._Package(ubuntu_name='libxcb-xinerama0'),
  ],

  cadence_palladium_pkgs:: [
    $._Package(ubuntu_name='rpcbind'),
  ],

  tsmc_pkgs:: [
    $._Package(ubuntu_name='ghostscript'),
  ],

  iscape_gui_pkgs:: [
    $._Package(ubuntu_name='lib32ncurses6'),
    $._Package(ubuntu_name='libxtst6:i386'),
  ],

  vmanager_pkgs:: [
    $._Package(ubuntu_name='ksh'),
  ],
  
  default_text_editors:: [
    $._Package(centos_name='vim'),
    $._Package(centos_name='emacs'),
  ],

  pd_team_custom_text_editors:: [
    $._Package(centos_name='gvim'),
    $._Package(centos_name='gedit'),
    $._Package(centos_name='kate'),
    $._Package(centos_name='neovim'),
  ],

  pd_team_data_transfer_pkgs:: [
    $._Package(centos_name='cpio'),
    $._Package(centos_name='wget'),
    $._Package(centos_name='pigz'),
  ],

  pd_team_parsing_pkgs:: [
    $._Package(centos_name='bison'),
    $._Package(centos_name='ocaml-csv'),
    $._Package(centos_name='ocaml-csv-devel'),
    $._Package(centos_name='xmlto'),
    $._Package(centos_name='flex'),
    $._Package(centos_name='graphviz'),
    $._Package(centos_name='goffice08'),
  ],

  pd_team_shell_pkgs:: [
    $._Package(centos_name='bash-completion'),
    $._Package(centos_name='dialog'),
    $._Package(centos_name='mosh'),
    $._Package(centos_name='screen'),
  ],

  pd_team_tech_doc_pkgs:: [
    $._Package(centos_name='asciidoc'),
    $._Package(centos_name='gettext'),
    $._Package(centos_name='qpdfview'),
  ],

  pd_team_file_comparison_pkgs:: [
    $._Package(centos_name='diffutils'),
    $._Package(centos_name='meld'),
  ],

  pd_team_file_filtering_pkgs:: [
    $._Package(centos_name='ripgrep'),
    $._Package(centos_name='tree'),
  ],

  permissions_pkgs:: [
    $._Package(centos_name='sudo'),
  ],

  // required to run bazel targets
  // python3 still needs to be installed under /usr/bin to initialize bazel
  // https://github.com/bazelbuild/rules_python/issues/691
  // but /usr/bin/python3 is not used to execute targets since the
  // python3 hermetic toolchain is used instead
  bazel_workflow_pkgs:: [
    $._Package(centos_name='python3'),
  ],

  // required to run gh/git commands
  // git is not installed via yum because the default version in centos
  // is too old and does not work with gh. Build a later version of git
  // from source and include it in the container instead.
  git_workflow_pkgs:: [
    $._Package(centos_name='gh'),
  ],

  // required packages for gee to run
  gee_pkgs:: [
    $._Package(centos_name='curl'),
    $._Package(centos_name='meld'),
    $._Package(centos_name='jq'),
    $._Package(centos_name='dialog'),
    $._Package(centos_name='vim'),
  ] + $.git_workflow_pkgs,

  synopsys_dft_shell_pkgs:: [
    $._Package(centos_name='lm_sensors-libs'),
  ],

  synopsys_testmax_atpg_pkgs:: [
    $._Package(centos_name='libxcb'),
    $._Package(centos_name='libxcb-devel'),
    $._Package(centos_name='xcb-util'),
    $._Package(centos_name='xcb-util-devel'),
  ],

  synopsys_icv_pkgs:: [
    $._Package(centos_name='snappy'),
    $._Package(centos_name='snappy-devel'),
  ],

  ausdia_timevision_pkgs:: [
    $._Package(centos_name='xorg-x11-utils'),
  ],

  ansys_edt_pkgs:: [
    $._Package(centos_name='mesa-dri-drivers'),
  ],

  // packages for the PD tools:
  // genus, innovus, tempus, conformal, voltus
  cadence_common_pkgs:: [
    $._Package(centos_name='csh'),
    $._Package(centos_name='ksh'),
    $._Package(centos_name='tcl'),
    $._Package(centos_name='tcsh'),
    $._Package(centos_name='libXScrnSaver'),
    $._Package(centos_name='glibc.i686'),
    $._Package(centos_name='elfutils-libelf.i686'),
    $._Package(centos_name='mesa-libGL.i686'),
    $._Package(centos_name='mesa-libGLU.i686'),
    $._Package(centos_name='motif'),
    $._Package(centos_name='motif.i686'),
    $._Package(centos_name='libXp'),
    $._Package(centos_name='libXp.i686'),
    $._Package(centos_name='libpng.i686'),
    $._Package(centos_name='libjpeg-turbo.i686'),
    $._Package(centos_name='expat.i686'),
    $._Package(centos_name='glibc-devel.i686'),
    $._Package(centos_name='glibc-devel'),
    $._Package(centos_name='redhat-lsb.i686'),
    $._Package(centos_name='redhat-lsb'),
    $._Package(centos_name='xterm'),
  ],

  quantus_pkgs:: [
    $._Package(centos_name='gdb'),
  ] + $.cadence_common_pkgs,

  keysight_pathwave_ads_pkgs:: [
    $._Package(centos_name='libxkbcommon-x11'),
  ],

  python3_build_pkgs:: [
    $._Package(centos_name='gcc'),
    $._Package(centos_name='openssl-devel'),
    $._Package(centos_name='bzip2-devel'),
    $._Package(centos_name='libffi-devel'),
  ],

  // This is the list of packages required for make-all and make-install.
  // Whenever make failed to build or install, an error message displayed
  // the missing package that needed to be installed.
  git2_build_pkgs:: [
    $._Package(centos_name='asciidoc'),
    $._Package(centos_name='libcurl-devel'),
    $._Package(centos_name='autoconf'),
    $._Package(centos_name='gcc'),
    $._Package(centos_name='curl'),
    $._Package(centos_name='openssl-devel'),
    $._Package(centos_name='expat-devel'),
    $._Package(centos_name='xmlto'),
  ],

  cjfs_pkgs:: [
    $._Package(centos_name='google-cloud-cli'),
  ],
}
