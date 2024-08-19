#!/bin/bash

set -e

template_file="/etc/yum.repos.d/local.template"
upstream_template_file="/etc/yum.repos.d/upstream.template"

# Configurable print
exec 3>/dev/null

for i in "$@"
do
case $i in
    # --repodir="path/to/repo:priority"
    --repodir=*)
        arg="${i#*=}"
        repodir="${arg%:*}"
        priority="${arg#*:}"
        echo "Running createrepo on $repodir with priority $priority" >&3 2>&3
        createrepo --compatibility "$repodir" >&3 2>&3
        if [ -n "$priority" ]; then
            sed -e "s|{{.num}}|$priority|g" "$template_file" > "/etc/yum.repos.d/local-$priority.repo"
        fi
        shift
        ;;
    --upstream-repo=*)
        priority="${i#*=}"
        echo "Adding upstream repo" >&3 2>&3
        sed -e "s|{{.num}}|$priority|g" "$upstream_template_file" > "/etc/yum.repos.d/upstream.repo"
        shift
        ;;
    #--install-dep='pkg = version'
    --install-dep=*)
        dep="${i#*=}"
        dnf install -y -q "$dep" >&3 2>&3
        shift
        ;;
    # --user='id:guid' --path='path'
    --user=*)
        user="${i#*=}"
        shift
        ;;
    --path=*)
        path_to_fix="${i#*=}"
        shift
        ;;
    --print-to-stderr)
        exec 3>&2
        shift
        ;;
    --print-to-stdout)
        exec 3>&1
        shift
        ;;
esac
done

# Treat every other argument as a command + args to run
command "$@"
ret=$?

# Change ownership of files back to the user
if [ -n "$user" ] && [ -n "$path_to_fix" ]; then
    chown -R "$user" "$path_to_fix" >&3 2>&3
fi

exit $ret
