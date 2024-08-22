FROM mcr.microsoft.com/azurelinux/base/core:3.0

# Tag the layers so we can clean up all the containers associated with a build directory
LABEL marinertoolchain=mockmockmock

RUN tdnf makecache
RUN tdnf install -y createrepo_c
RUN tdnf install -y dnf-utils

# Refresh the cache (use date to force a cache refresh each hour)
RUN dnf makecache -y --enablerepo=* && echo "Hello world"

# Copy in create_repos_and_run.sh and place on path
COPY [ "./create_repos_and_run.sh", \
       "/usr/bin/" ]

COPY [ "./local.template", \
       "/etc/yum.repos.d/" ]

COPY [ "./upstream.template", \
       "/etc/yum.repos.d/" ]
