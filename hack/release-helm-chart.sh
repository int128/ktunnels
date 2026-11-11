#!/bin/bash
set -eux -o pipefail

: "$IMG"

chart_name=ktunnels
chart_registry="oci://ghcr.io/$GITHUB_REPOSITORY/charts"
chart_version="${GITHUB_REF_NAME#v}"

# Set the default image
yq -i '.manager.image.repository = env(IMG)' charts/chart/values.yaml
helm package charts/chart --app-version "$chart_version" --version "$chart_version"

if [[ $GITHUB_REF_TYPE = tag ]]; then
  helm push "${chart_name}-${chart_version}.tgz" "$chart_registry"
fi
