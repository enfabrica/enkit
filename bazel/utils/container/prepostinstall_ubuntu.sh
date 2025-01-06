#!/bin/bash
set -o pipefail -o errexit -o errtrace -o nounset

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
echo "No custom preinstall or postinstall script defined. Skipping this step."
echo ""
