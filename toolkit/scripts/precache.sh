#!/bin/bash
# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# Usage: precache.sh </path/to/snapshot.json> <cache_working_dir> <cache_dir> <downloaded_files.txt> <repo_base_urls>...
snapshot_path="$1"
[[ -z "$snapshot_path" ]] && echo "snapshot_path is required" && exit 1
shift

cache_working_dir="$1"
[[ -z "$cache_working_dir" ]] && echo "cache_working_dir is required" && exit 1
shift

cache_dir="$1"
[[ -z "$cache_dir" ]] && echo "cache_dir is required" && exit 1
shift

downloaded_files="$1"
[[ -z "$downloaded_files" ]] && echo "downloaded_files is required" && exit 1
shift

base_urls=("$@")
[[ ${#base_urls[@]} -eq 0 ]] && echo "base_urls is required" && exit 1

mkdir -p "$cache_working_dir"
mkdir -p "$cache_dir"
rm -f "$downloaded_files"

. /etc/os-release
if [[ "$ID" == "mariner" ]]; then
    REPOQUERY_OS_ARGS="-y -q --disablerepo=*"
else
    REPOQUERY_OS_ARGS="--show-duplicates --tempcache"
fi


# Init the cache
mkdir -p "$cache_working_dir"
mkdir -p "$cache_dir"
rm -f "$downloaded_files"

# For each base url, pull the meta data and write it to the repo summary file
echo "" > $cache_working_dir/repo.txt
for base_url in "${base_urls[@]}"; do
    repo_unique="repo-$(echo $(((RANDOM%99999)+10000)))"
    repo_name="mariner-precache-$repo_unique"
    echo "Querying repo $base_url via 'repoquery $REPOQUERY_OS_ARGS --repofrompath=$repo_name,$base_url -a --qf="%{location}" >> $cache_working_dir/repo.txt'"
    repoquery $REPOQUERY_OS_ARGS --repofrompath=$repo_name,$base_url -a --qf="%{location}" >> $cache_working_dir/repo.txt || exit 1
done

touch "$downloaded_files"
function download_rpm() {
    local name="$1"
    rpm_name="$name.rpm"
    # '/' is important here, otherwise we might match a similarly named package. The '/' guarantees we match the start of the name.
    rpm_url=$(grep "/$rpm_name" $cache_working_dir/repo.txt | head -n 1)
    if [[ -z "$rpm_url" ]]; then
        return
    fi

    # We can't use -O with wget since that resets the timestamp. We want to use -N to only download if the file is newer so we
    # need to use cd to the cache directory and then wget the file.
    if ! output=$(cd "$cache_dir" && wget -nv -N "$rpm_url" 2>&1); then
        echo "Failed to pre-cache $rpm_url"
        rm -f "$cache_dir/$rpm_name"
        return
    else
        # Output will contain the url + path only if it is downloaded. If it is not downloaded, it will be an empty string. Write all files we
        # successfully downloaded to the downloaded_files file.
        echo "$output" 
        if [[ -n "$output" ]]; then
            echo "Successfully pre-cached $rpm_url"
            echo "$output" >> "$downloaded_files"
        fi
    fi
}

# For each rpm in the snapshot .json file, download it to the cache directory. We format the output as "Name-Version.Distribution.Architecture"
for full_name in $(jq -r '.Repo[] | "\(.Name)-\(.Version).\(.Distribution).\(.Architecture)"' "$snapshot_path" | sort -u); do
    download_rpm "$full_name" &
done

wait
echo "Files in $downloaded_files"
echo "Pre-cached $(wc -l "$downloaded_files") packages"
