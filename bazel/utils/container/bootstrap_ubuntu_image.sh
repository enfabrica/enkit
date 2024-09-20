#!/bin/bash
set -o pipefail -o errexit -o nounset
USER=$(whoami)

readonly outfile="$1"
readonly pkgs="${@:2}"
tmp_dir=$(mktemp -d)
mkdir "$tmp_dir/root"
mkdir "$tmp_dir/debootstrap"
tmp_root="$tmp_dir/root"
cleanup() {
    echo "Cleaning up tmp directory $tmp_dir"
    echo ""
    sudo rm -rf $tmp_dir
}
trap cleanup EXIT

entry_per_line="debootstrap/required debootstrap/deburis debootstrap/debpaths"
all_entries_one_line="debootstrap/base"
debootstrap_log="debootstrap/debootstrap.log"
touch "$tmp_dir/$all_entries_one_line.tmp"
for p in $pkgs
do
    echo "Unpacking $p into $tmp_root"
    echo ""
    sudo tar -xf $p -C $tmp_root "var/"
    sudo tar \
    --exclude="debootstrap/required" \
    --exclude="debootstrap/debootstrap.log" \
    --exclude="debootstrap/base" \
    --exclude="debootstrap/deburis" \
    --exclude="debootstrap/debpaths" -xf $p -C $tmp_root
    
    for f in $entry_per_line $debootstrap_log
    do
        sudo tar --to-command="sudo tee -a $tmp_dir/$f.tmp" -xf $p $f
    done
    echo " " | sudo tee -a "$tmp_dir/$all_entries_one_line.tmp"
    sudo tar --to-command="sudo tee -a $tmp_dir/$all_entries_one_line.tmp" -xf $p $all_entries_one_line
done

for f in $entry_per_line
do
    # Remove all duplicated entries
    sudo cat "$tmp_dir/$f.tmp" | sort | uniq | sudo tee -a "$tmp_root/$f" 
done

# Parse the space-delimited single line into one word per line
# then remove all duplicates before transforming multiple lines
# back into a space-delimited single line file.
# For the debootstrap/base file, remove the leading ' ' character
# or else it will break debootstrap during the unpacking phase.
sudo cat "$tmp_dir/$all_entries_one_line.tmp" | tr ' ' '\n' | sort | uniq | paste -sd ' ' | cut -c2- | sudo tee -a $tmp_root/$all_entries_one_line
sudo cp "$tmp_dir/$debootstrap_log.tmp" $tmp_root/$debootstrap_log

sudo tar -zcf $outfile -C $tmp_root .
# Change the ownership of the file back to a regular user
# or else bazel will fail because bazel does not treat
# output files owned by root as valid.
sudo chown $USER:$USER $outfile

