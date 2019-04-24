#!/usr/bin/env bash

DIR=${DIR:-$(cd $(dirname "$0")/.. && pwd)}
NAME=${NAME:-$(ls $DIR/deploy/olm-catalog)}

x=( $(echo $NAME | tr '-' ' ') )
DISPLAYNAME=${DISPLAYNAME:=${x[*]^}}

LATEST=$(find $DIR/deploy/olm-catalog -name '*version.yaml' | sort -n | sed "s/^.*\/\([^/]..*\).clusterserviceversion.yaml$/\1/" | tail -1)

indent() {
  INDENT="      "
  sed "s/^/$INDENT/" | sed "s/^${INDENT}\($1\)/${INDENT:0:-2}- \1/"
}

CRD=$(cat $(ls $DIR/deploy/crds/*crd.yaml) | grep -v -- "---" | indent apiVersion)
CSV=$(cat $(find $DIR/deploy/olm-catalog -name '*version.yaml') | indent apiVersion)

cat <<EOF | sed 's/^  *$//'
kind: ConfigMap
apiVersion: v1
metadata:
  name: $NAME

data:
  customResourceDefinitions: |-
$CRD
  clusterServiceVersions: |-
$CSV
  packages: |-
    - packageName: $NAME
      channels:
      - name: alpha
        currentCSV: $LATEST
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: $NAME
spec:
  configMap: $NAME
  displayName: $DISPLAYNAME
  publisher: Red Hat
  sourceType: internal
EOF
