#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Basic script to apply all patches listed in a spec file in the correct order.
# Use: Extract the source archive into the desired working folder

# $1 - path to spec file
# $2 - path to output folder with base sources applied (and git repo init'd)
set -e

[[ -n $1 ]] || { echo "Need path to spec file"; exit 1; }
[[ -n $2 ]] || { echo "Need path to output dir"; exit 1; }

parsed_spec=$(rpmspec --srpm --parse --define "with_check 0" $1 )
patch_list=$(echo "$parsed_spec" | grep "%patch" | sed -n -r 's/.*%patch([0-9]*).*/\1/p')
echo "$parsed_spec" > spec.txt
echo "$patch_list" > patches.txt
echo "Done parsing"
for patch in $patch_list
do
    echo "Applying $patch"
    # From "Patch##:   ####-my-patch.patch" get "/my/current/dir" "/path/to/spec/" "####-my-patch.patch" to for a full path
    patch_path=$(pwd)/$(dirname $1)/$(echo "$parsed_spec" | grep "Patch${patch}:.*\.patch" | sed -n -r 's/^.*Patch[0-9]*:\s*(.*)$/\1/p')
    # From "Patch##:   ####-my-patch.patch" get "my-patch"
    patch_name=$(echo "$parsed_spec" | grep "Patch${patch}:.*\.patch" | sed -n -r 's/^.*Patch[0-9]*:\s*[0-9]*-*(.*)\.patch$/\1/p')
    ( cd $2; git am $patch_path || { echo "FALLING BACK TO GIT APPLY!"; git apply $patch_path && echo "GIT ADD!" && git add -A && echo "GIT COMMIT!" && git commit -m "$patch_name"; } || { echo "FAILED TO APPLY!"; exit 1;} )
done
echo "$parsed_spec" > spec.txt