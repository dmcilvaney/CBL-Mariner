FROM mcr.microsoft.com/azurelinux/base/core:3.0

# Tag the layers so we can clean up all the containers associated with a build directory
LABEL marinertoolchain=mockmockmock

RUN tdnf makecache
Run tdnf install -y azurelinux-rpm-macros
RUN tdnf install -y createrepo_c
RUN tdnf install -y dnf-utils

# Refresh the cache (use date to force a cache refresh each hour)
ARG CACHEBUST
# Save the cachebust value in the image to a file so it invalidates the cache
RUN echo $CACHEBUST > /etc/CACHEBUST

RUN dnf makecache -y --enablerepo=*

# Copy in create_repos_and_run.sh and place on path
COPY [ "./create_repos_and_run.sh", \
       "/usr/bin/" ]

COPY [ "./local.template", \
       "/etc/yum.repos.d/" ]

COPY [ "./upstream.template", \
       "/etc/yum.repos.d/" ]
