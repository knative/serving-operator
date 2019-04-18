#!/usr/bin/env bash

DIR=${DIR:-$(cd $(dirname "$0")/.. && pwd)}
SRC=${DIR}/deploy/resources
TGT=${DIR}/deploy/crds

function extract {
  local file=$1
  local base=$(basename -s .yaml $file)
  
  csplit -q -f ${TGT}/${base} -b "_%02d_crd.yaml" ${SRC}/${file} "/^---$/" "{*}"
  for i in $(ls ${TGT}/${base}*); do
    if grep "kind: CustomResourceDefinition" $i >/dev/null; then
      echo $i
    else
      rm $i
    fi
  done
}

for i in $(ls ${SRC}); do
  extract $i
done
