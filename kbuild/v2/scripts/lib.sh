#!/bin/sh

# common helpers

get_kernel_base() {
    local deb_dir="$1"

    # discover KERNEL_BASE from the linux-headers .deb file name
    local headers_deb="${deb_dir}/linux-headers-*_*_all.deb"
    if [ ! -r $headers_deb ] ; then
        echo "ERROR: unable to find common header .deb: $headers_deb"
        exit 1
    fi

    local file_name=$(basename $(ls $headers_deb))
    local kernel_base=${file_name#linux-headers-}
    kernel_base=${kernel_base%%_*_all.deb}
    echo -n $kernel_base
}

get_deb_version() {
    local deb_dir="$1"

    # discover DEB_VERSION from the linux-headers .deb file name
    local headers_deb="${deb_dir}/linux-headers-*_*_all.deb"
    if [ ! -r $headers_deb ] ; then
        echo "ERROR: unable to find common header .deb: $headers_deb"
        exit 1
    fi

    local file_name=$(basename $(ls $headers_deb))
    local deb_version=${file_name%_all.deb}
    deb_version=${deb_version##*_}
    echo -n $deb_version
}

upload_artifact() {
    local archive="$1"
    local astore_path="$2"
    local arch="$3"
    local tag="$4"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to find archive: $archive"
        exit 1
    fi

    # upload archive to astore
    enkit astore upload "${archive}@${astore_path}" -a $arch -t "$tag"

    echo "Upload sha256sum:"
    sha256sum "$archive"

}
