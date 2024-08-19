#!/bin/bash

set -e

#arg1,2 may be --user='id:guid' --path='path'
for i in "$@"
do
case $i in
    --user=*)
    user="${i#*=}"
    shift
    ;;
    --path=*)
    path_to_fix="${i#*=}"
    shift
    ;;
    *)
    ;;
esac
done

deps_dir=/deps


# Install all deps if directory exists and has *.rpm files
if [ -d "$deps_dir" ] && [ "$(ls -A $deps_dir)" ]; then
    rpm -ivh $deps_dir/*.rpm
fi

# Treat every other argument as a command + args to run
command "$@"
ret=$?

# Change ownership of files back to the user
if [ -n "$user" ] && [ -n "$path_to_fix" ]; then
    chown -R "$user" "$path_to_fix"
fi

exit $ret
