declare -A osInfo;
osInfo[/etc/redhat-release]=yum
osInfo[/etc/arch-release]=pacman
osInfo[/etc/gentoo-release]=emerge
osInfo[/etc/SuSE-release]=zypp
osInfo[/etc/debian_version]=apt-get

PACKAGES="-y libpam-script"
for f in "${!osInfo[@]}"
do
    if [[ -f $f ]];then
        sudo ${osInfo[$f]} install $PACKAGES
    fi
done
