local util = import 'bazel/utils/container/muk/util.libsonnet';
local package_lists = import 'bazel/utils/container/muk/packages.libsonnet';

{
  base_image: 'us-docker.pkg.dev/enfabrica-container-images/third-party-prod/docker.io/library/ubuntu@sha256:e722c7335fdd0ce77044ab5942cb1fbd2b5f60d1f5416acfcdb0814b2baf7898',
  astore_files: [
    util.AstoreFile('sgskwx22xjs7ogub543dhg35dpihvznd', 'infra/dev_container/libprotobuf7_i386.deb'),
    util.AstoreFile('8r2xz6y6t7r4jebxjdopi6gofnsyt8br', 'infra/dev_container/chrome-remote-desktop_current_amd64.deb'),
    util.AstoreFile('oj5arozr8csxi7bspczjgu6v5y42cykh', 'infra/dev_container/gh.deb'),
    util.AstoreFile('x5gpmb5fnwgf4giyguxgnn2ci2m562g6', 'tools/cudnn-local-repo.deb'),
    util.AstoreFile('rvunwsv7fvtoqwz3x24ge3qpko5u352a', 'infra/dev_container/cuda_12_linux.run'),
  ],
  actions: [
    { dpkg_add_arch: { architecture: 'i386' } },
    { apt_add_ppa: { name: 'git-core/ppa' } },
    { apt_install: { packages: package_lists.DevBasePackages() } },
    {
      apt_add_repo: {
        name: 'google_cloud_sdk',
        binary_url: 'https://packages.cloud.google.com/apt',
        distribution: 'cloud-sdk',
        components: [
          'main',
        ],
        signing_key: {
          name: 'cloud.google',
          url: 'https://packages.cloud.google.com/apt/doc/apt-key.gpg',
        },
      },
    },
    {
      apt_add_repo: {
        name: 'hashicorp',
        binary_url: 'https://apt.releases.hashicorp.com',
        distribution: 'focal',
        components: [
          'main',
        ],
        repo_options: [
          {
            name: 'arch',
            values: [
              'amd64',
            ],
          },
        ],
        signing_key: {
          name: 'hashicorp',
          url: 'https://apt.releases.hashicorp.com/gpg',
        },
      },
    },
    util.Command('wget -O /usr/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/download/v1.9.0/bazelisk-linux-amd64'),
    util.Command('chmod 0777 /usr/bin/bazelisk'),
    util.Command('ln -sf /usr/bin/bazelisk /usr/bin/bazel'),
    util.Command('sh /astore/cuda_12_linux.run --silent --installpath=/usr/local/cuda-12.1 --toolkit --no-man-page --no-opengl-libs --no-drm'),
    util.Command('rm -rf /usr/local/cuda/extras /usr/local/cuda/libnvvp /usr/local/cuda/nsight*'),
    {
      apt_install: {
        // TODO(bbhuynh): Should some of this get pushed back into packages.libsonnet?
        // TODO(bbhuynh): Can all apt installs be done after adding apt repos?
        packages: [
          'clangd',
          'google-cloud-sdk',
          'google-cloud-sdk-app-engine-python',
          'google-cloud-sdk-gke-gcloud-auth-plugin',
          'kubectl',
          'packer',
          'unzip',
          '/astore/libprotobuf7_i386.deb',
          '/astore/chrome-remote-desktop_current_amd64.deb',
          '/astore/gh.deb',
          '/astore/cudnn-local-repo.deb',
        ],
      },
    },
    util.Command('cp /var/cudnn-local-repo-ubuntu2004-8.8.1.3/cudnn-local-*-keyring.gpg /usr/share/keyrings/'),
    { apt_install: { packages: ['libcudnn8-dev'] } },
    util.Command('locale-gen en_US.UTF-8'),
    util.Command('chmod -x /etc/update-motd.d/*'),
    util.Command("echo 'dash dash/sh boolean false' | debconf-set-selections"),
    util.Command('dpkg-reconfigure dash'),
    util.Command('mkdir -p /lib/modules'),
  ],
}
