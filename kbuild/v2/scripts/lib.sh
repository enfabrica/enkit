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

deb_get_kernel_version() {
    local deb_dir="$1"
    local flavour="$2"
    local kernel_base=$(get_kernel_base $deb_dir)

    if [ -z "$kernel_base" ] ; then
        echo "ERROR: unable to discover kernel base string"
        exit 1
    fi

    local kernel_version="${kernel_base}-${flavour}"

    echo -n "$kernel_version"
}

upload_artifact() {
    local archive="$1"
    local astore_path="$2"
    local arch="$3"
    local tag="$4"
    local archive_json="$5"

    if [ ! -r "$archive" ] ; then
        echo "ERROR: unable to find archive: $archive"
        exit 1
    fi

    # upload archive to astore
    "$RT_ENKIT" astore upload "${archive}@${astore_path}" -a $arch -t "$tag" -m "${archive_json}"

    # add the sha256sum to the resulting artifact meta-data
    local sha256=$(sha256sum "$archive" | awk '{ print $1 }')

    cat "${archive_json}" | jq '.Artifacts[0] + { "sha256": "'"$sha256"'" }' > "${archive_json}.tmp"
    mv -f "${archive_json}.tmp" "$archive_json"
}
