#!/bin/bash
sudo rm -rf /tmp/azl-toolkit/test-overlay
mkdir -p /tmp/azl-toolkit/test-overlay/overlay1/upper
mkdir -p /tmp/azl-toolkit/test-overlay/overlay1/work
mkdir -p /tmp/azl-toolkit/test-overlay/overlay2/upper
mkdir -p /tmp/azl-toolkit/test-overlay/overlay2/work
mkdir -p /tmp/azl-toolkit/test-overlay/overlay3/upper
mkdir -p /tmp/azl-toolkit/test-overlay/overlay3/work
mkdir -p /tmp/azl-toolkit/test-overlay/overlay4/upper
mkdir -p /tmp/azl-toolkit/test-overlay/overlay4/work

# docker 'run' '--rm' '--network' 'host'
# '--mount' 'type=volume,dst=/repos/2,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/fake_pmc,upperdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay986679399/upper,workdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay986679399/work"'
# '--mount' 'type=volume,dst=/repos/0,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS,upperdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay3011187550/upper,workdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay3011187550/work"'
# '--mount' 'type=volume,dst=/repos/1,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/1,upperdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay2496246419/upper,workdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay2496246419/work"'
# '--mount' 'type=volume,dst=/repos/upstream,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-cache,upperdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay2490632419/upper,workdir=/tmp/azl-toolkit/docker-overlay2890884628/overlay2490632419/work"'
# '-v' '/etc/passwd:/etc/passwd:ro' '-v' '/etc/group:/etc/group:ro' '-v' '/etc/shadow:/etc/shadow:ro' 'mcr.microsoft.com/azurelinux/local_builder/cache'
# 'create_repos_and_run.sh' '--print-to-stderr' '--repodir=/repos/2:2' '--repodir=/repos/0:0' '--repodir=/repos/1:1' '--upstream-repo-priority=3'
# 'repoquery' '--disablerepo=*' '--enablerepo=*' '--qf' 'PROVIDES_LOOKUP:	%{name}	%{evr}	%{repoid}' '--whatprovides' 'pkgconfig(termcap)'

docker 'run' '--rm' -it '--network' 'host' \
    '--mount' 'type=volume,dst=/repos/2,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/fake_pmc,upperdir=/tmp/azl-toolkit/test-overlay/overlay1/upper,workdir=/tmp/azl-toolkit/test-overlay/overlay1/work"'\
    '--mount' 'type=volume,dst=/repos/0,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS,upperdir=/tmp/azl-toolkit/test-overlay/overlay2/upper,workdir=/tmp/azl-toolkit/test-overlay/overlay2/work"'\
    '--mount' 'type=volume,dst=/repos/1,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/1,upperdir=/tmp/azl-toolkit/test-overlay/overlay3/upper,workdir=/tmp/azl-toolkit/test-overlay/overlay3/work"'\
    '--mount' 'type=volume,dst=/repos/upstream,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-cache,upperdir=/tmp/azl-toolkit/test-overlay/overlay4/upper,workdir=/tmp/azl-toolkit/test-overlay/overlay4/work"'\
    '-v' '/etc/passwd:/etc/passwd:ro' '-v' '/etc/group:/etc/group:ro' '-v' '/etc/shadow:/etc/shadow:ro' 'mcr.microsoft.com/azurelinux/local_builder/cache' \
    'create_repos_and_run.sh' '--print-to-stderr' '--repodir=/repos/2:2' '--repodir=/repos/0:0' '--repodir=/repos/1:1' '--upstream-repo-priority=3' \
    'bash'


#'repoquery' '--disablerepo=*' '--enablerepo=*' '--qf' 'PROVIDES_LOOKUP:	%{name}	%{evr}	%{repoid}' '--whatprovides' 'pkgconfig(termcap)'
