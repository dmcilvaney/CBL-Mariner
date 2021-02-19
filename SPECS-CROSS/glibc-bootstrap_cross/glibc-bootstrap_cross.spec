%global security_hardening nonow
%define glibc_target_cpu %{_build}
%define debug_package %{nil}

# Globals which should be in a macro file.
# These should be set programatically in the future.
%global _host_arch      x86_64
%global _target_arch    aarch64

%global _tuple          %{_target_arch}-%{_vendor}-linux-gnu
%global _cross_name     %{_target_arch}-%{_vendor}-linux-gnu

# Folders which should be in our macro file
%global _opt                /opt/
%global _crossdir           /opt/cross/

# Generally we include '/usr' in most paths.
# Can we also use '/usr' for our paths? This will bring us in line with the
# %%configure macro which sets these.
%global _bindir            /bin
%global _sbindir           /sbin
%global _libdir            /lib
%global _lib64dir          /lib64
%global _libexecdir        /libexec
%global _datadir           /share
%global _docdir            /share/doc
%global _includedir        /include
%global _infodir           /share/info
%global _mandir            /share/man
%global _oldincludedir     /include

# If we want our cross compile aware packges to also support native, we
# need logic to switch modes something like this:
%if %{_target_arch} != %{_host_arch}
%global _cross_prefix       %{_crossdir}%{_tuple}/
%global _cross_sysroot      %{_crossdir}%{_tuple}/sysroot/
%global _cross_includedir   /usr/%{_host}/%{_tuple}/include/
%global _cross_infodir      %{_crossdir}%{_tuple}/share/info
%global _cross_bindir       %{_tuple}/bin
%global _cross_libdir       %{_tuple}/lib
%global _tuple_name         %{_tuple}-

%global __strip %{_cross_prefix}%{_bindir}/%{_tuple_name}strip
%else
%global _cross_prefix       %{nil}
%global _cross_sysroot      %{nil}
%global _cross_includedir   %{_includedir}
%global _cross_infodir      %{_infodir}
%global _cross_bindir       %{_bindir}
%global _cross_libdir       %{_libdir}
%global _tuple_name         %{nil}
%endif

Summary:        Main C library
Name:           %{_cross_name}-glibc-bootstrap
Version:        2.28
Release:        14%{?dist}
License:        LGPLv2+
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/System
URL:            https://www.gnu.org/software/libc
Source0:        https://ftp.gnu.org/gnu/glibc/glibc-%{version}.tar.xz
Patch0:         http://www.linuxfromscratch.org/patches/downloads/glibc/glibc-2.25-fhs-1.patch
Patch1:         glibc-2.24-bindrsvport-blacklist.patch
Patch2:         0002-malloc-arena-fix.patch
Patch3:         glibc-2.28-CVE-2018-19591.patch
Patch4:         CVE-2019-9169.patch
Patch5:         CVE-2016-10739.patch
Patch6:         CVE-2020-1752.patch
Patch7:         CVE-2020-10029.patch
# Only applicable on ARMv7 targets.
Patch8:         CVE-2020-6096.nopatch
# Only applicable on x32 targets.
Patch9:         CVE-2019-6488.nopatch
# Only applicable on PowerPC targets.
Patch10:        CVE-2020-1751.nopatch
# Marked by upstream/Ubuntu/Red Hat as not a security bug, no fix available
# Rationale: Exploit requires crafted pattern in regex compiler meant only for trusted content
Patch11:        CVE-2018-20796.nopatch
#Requires:       filesystem
Provides:       %{name}-common = %{version}-%{release}
Provides:       %{_cross_name}-rtld(GNU_HASH)
Provides:       %{_crossdir}%{_tuple}/sbin/ldconfig
BuildRequires:  %{_cross_name}-binutils
BuildRequires:  %{_cross_name}-kernel-headers
BuildRequires:  %{_cross_name}-gcc-bootstrap
AutoReqProv:    no
Conflicts:      %{_cross_name}-glibc
Conflicts:      %{_cross_name}-glibc-bootstrap
ExcludeArch:    armv7 ppc i386 i686

%description
This library provides the basic routines for allocating memory,
searching directories, opening and closing files, reading and
writing files, string handling, pattern matching, arithmetic,
and so on.

#Merge all of the packages for the bootstrap packages

# %%package devel
# Summary:        Header files for glibc
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}
# Provides:       %%{name}-headers = %%{version}-%%{release}
# Provides:       %%{name}-static = %%{version}-%%{release}
# Provides:       %%{name}-static%%{?_isa} = %%{version}-%%{release}

# %%description devel
# These are the header files of glibc.

# %%package lang
# Summary:        Additional language files for glibc
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}

# %%description lang
# These are the additional language files of glibc.

# %%package i18n
# Summary:        Additional internationalization files for glibc
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}

# %%description i18n
# These are the additional internationalization files of glibc.

# %%package iconv
# Summary:        gconv modules for glibc
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}

# %%description iconv
# These are gconv modules for iconv().

# %%package tools
# Summary:        tools for glibc
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}

# %%description tools
# Extra tools for glibc.

# %%package nscd
# Summary:        Name Service Cache Daemon
# Group:          Applications/System
# Requires:       %%{name} = %%{version}-%%{release}

# %%description nscd
# Name Service Cache Daemon

%prep
%setup -q -n glibc-%{version}
sed -i 's/\\$$(pwd)/`pwd`/' timezone/Makefile
%patch0 -p1
%patch1 -p1
%patch2 -p1
%patch3 -p1
%patch4 -p1
%patch5 -p1
%patch6 -p1
%patch7 -p1

install -vdm 755 %{_builddir}/%{name}-build
# do not try to explicitly provide GLIBC_PRIVATE versioned libraries
%define __find_provides %{_builddir}/%{name}-%{version}/find_provides.sh
%define __find_requires %{_builddir}/%{name}-%{version}/find_requires.sh

# create find-provides and find-requires script in order to ignore GLIBC_PRIVATE errors
cat > find_provides.sh << _EOF
#! /bin/sh
if [ -d /tools ]; then
/tools/lib/rpm/find-provides | grep -v GLIBC_PRIVATE
else
%{_lib}/rpm/find-provides | grep -v GLIBC_PRIVATE
fi
exit 0
_EOF
chmod +x find_provides.sh

cat > find_requires.sh << _EOF
#! /bin/sh
if [ -d /tools ]; then
/tools/lib/rpm/find-requires %{buildroot} %{glibc_target_cpu} | grep -v GLIBC_PRIVATE
else
%{_lib}/rpm/find-requires %{buildroot} %{glibc_target_cpu} | grep -v GLIBC_PRIVATE
fi
_EOF
chmod +x find_requires.sh
#___EOF

%build
# What flags do we want here?
CFLAGS=""
CXXFLAGS=""
#CFLAGS="`echo " %%{build_cflags} " | sed 's/-Wp,-D_FORTIFY_SOURCE=2//'`"
#CXXFLAGS="`echo " %%{build_cxxflags} " | sed 's/-Wp,-D_FORTIFY_SOURCE=2//'`"
export CFLAGS
export CXXFLAGS

# Need to make some temp directories to put files into, we don't want to polute our
# build machines directores and we shouldn't be touching BUILDROOT yet.
TEMP_SYSROOT="%{_builddir}/temp_sysroot/"
cp -r "%{_cross_sysroot}" "$TEMP_SYSROOT"

export PATH="%{_cross_prefix}%{_bindir}":$PATH

cd %{_builddir}/%{name}-build
../glibc-%{version}/configure \
            --prefix=/ \
            --build=%{_host_arch}-%{_vendor}-linux-gnu \
            --host=%{_tuple} \
            --target=%{_tuple} \
            --with-sysroot="$TEMP_SYSROOT" \
            --with-headers="$TEMP_SYSROOT/%{_includedir}" \
            --disable-multilib \
            libc_cv_forced_unwind=yes \
            --disable-werror

make %{?_smp_mflags} DESTDIR=$TEMP_SYSROOT install-bootstrap-headers=yes install-headers
make %{?_smp_mflags} csu/subdir_lib
# Depending on the state of the sysroot, /lib may already exist
mkdir -p "$TEMP_SYSROOT/%{_libdir}"
install csu/crt1.o csu/crti.o csu/crtn.o "$TEMP_SYSROOT/%{_libdir}"
%{_cross_name}-gcc -nostdlib -nostartfiles -shared -x c /dev/null -o "$TEMP_SYSROOT%{_libdir}/libc.so"

# Sometimes we have false "out of memory" make error
# just rerun/continue make to workaroung it.
#make %%{?_smp_mflags} || make %%{?_smp_mflags} || make %%{?_smp_mflags}

%install
cd %{_builddir}/%{name}-build
make %{?_smp_mflags} DESTDIR="%{buildroot}%{_cross_sysroot}" install-bootstrap-headers=yes install-headers
touch %{buildroot}%{_cross_sysroot}%{_includedir}/gnu/stubs.h
mkdir %{buildroot}%{_cross_sysroot}/%{_libdir}
install csu/crt1.o csu/crti.o csu/crtn.o ../temp_sysroot/%{_libdir}/libc.so %{buildroot}%{_cross_sysroot}/%{_libdir}

# #       Do not remove static libs
# pushd %%{_builddir}/glibc-build
# #       Create directories
# make install_root=%%{buildroot} install
# install -vdm 755 %%{buildroot}%%{_sysconfdir}/ld.so.conf.d
# install -vdm 755 %%{buildroot}%%{_var}/cache/nscd
# install -vdm 755 %%{buildroot}%%{_libdir}/locale
# cp -v ../%%{name}-%%{version}/nscd/nscd.conf %%{buildroot}%%{_sysconfdir}/nscd.conf
# #       Install locale generation script and config file
# cp -v %%{SOURCE2} %%{buildroot}%%{_sysconfdir}
# cp -v %%{SOURCE1} %%{buildroot}/sbin
# #       Remove unwanted cruft
# rm -rf %%{buildroot}%%{_infodir}
# #       Install configuration files

# Spaces should not be used in nsswitch.conf in the begining of new line
# Only tab should be used as it expects the same in source code.
# Otherwise "altfiles" will not be added. which may cause dbus.service failure
# cat > %{buildroot}%{_sysconfdir}/nsswitch.conf <<- "EOF"
# #       Begin /etc/nsswitch.conf

# 	passwd: files
# 	group: files
# 	shadow: files

# 	hosts: files dns
# 	networks: files

# 	protocols: files
# 	services: files
# 	ethers: files
# 	rpc: files
# #       End /etc/nsswitch.conf
# EOF
# cat > %{buildroot}%{_sysconfdir}/ld.so.conf <<- "EOF"
# #       Begin /etc/ld.so.conf
# 	%{_prefix}/local/lib
# 	/opt/lib
# 	include %{_sysconfdir}/ld.so.conf.d/*.conf
# EOF
# popd
#%find_lang %{name} --all-name
# pushd localedata
# # Generate out of locale-archive an (en_US.) UTF-8 locale
# mkdir -p %{buildroot}%{_lib}/locale
# I18NPATH=. GCONV_PATH=../../glibc-build/iconvdata LC_ALL=C ../../glibc-build/locale/localedef --no-archive --prefix=%{buildroot} -A ../intl/locale.alias -i locales/en_US -c -f charmaps/UTF-8 en_US.UTF-8
# mv %{buildroot}%{_lib}/locale/en_US.utf8 %{buildroot}%{_lib}/locale/en_US.UTF-8
# popd
# to do not depend on /bin/bash
# sed -i 's@#! /bin/bash@#! /bin/sh@' %{buildroot}%{_bindir}/ldd
# sed -i 's@#!/bin/bash@#!/bin/sh@' %{buildroot}%{_bindir}/tzselect

# %check
# cd %{_builddir}/glibc-build
# make %{?_smp_mflags} check ||:
# # These 2 persistant false positives are OK
# # XPASS for: elf/tst-protected1a and elf/tst-protected1b
# [ `grep ^XPASS tests.sum | wc -l` -ne 2 -a `grep "^XPASS: elf/tst-protected1[ab]" tests.sum | wc -l` -ne 2 ] && exit 1 ||:

# # FAIL (intermittent) in chroot but PASS in container:
# # posix/tst-spawn3 and stdio-common/test-vfprintf
# n=0
# grep "^FAIL: posix/tst-spawn3" tests.sum >/dev/null && n=$((n+1)) ||:
# grep "^FAIL: stdio-common/test-vfprintf" tests.sum >/dev/null && n=$((n+1)) ||:
# # FAIL always on overlayfs/aufs (in container)
# grep "^FAIL: posix/tst-dir" tests.sum >/dev/null && n=$((n+1)) ||:

# #https://sourceware.org/glibc/wiki/Testing/Testsuite
# grep "^FAIL: nptl/tst-eintr1" tests.sum >/dev/null && n=$((n+1)) ||:
# #This happens because the kernel fails to reap exiting threads fast enough,
# #eventually resulting an EAGAIN when pthread_create is called within the test.

# # check for exact 'n' failures
# [ `grep ^FAIL tests.sum | wc -l` -ne $n ] && exit 1 ||:

# %post -p /sbin/ldconfig
# %postun -p /sbin/ldconfig

%files
%defattr(-,root,root)
%license LICENSES
%{_cross_sysroot}%{_libdir}/*.so
%{_cross_sysroot}%{_libdir}/*.o
%{_cross_sysroot}%{_includedir}/*

%changelog
* Thu Dec 10 2020 Joe Schmitt <joschmit@microsoft.com> - 2.28-14
- Provide isa version of glibc-static.

* Mon Sep 28 2020 Ruying Chen <v-ruyche@microsoft.com> - 2.28-13
- Move some tools from glibc-tools and glibc-iconv to glibc and provide glibc-common
- Provide glibc-static and glibc-headers under glibc-devel

* Wed Jul 29 2020 Thomas Crain <thcrain@microsoft.com> - 2.28-12
- Ignore CVE-2018-20796, as it is not a security issue

* Wed Jul 29 2020 Emre Girgin <mrgirgin@microsoft.com> - 2.28-11
- Disable the debuginfo package for glibc, and use unstripped binaries instead.

* Fri Jun 26 2020 Ruying Chen <v-ruyche@microsoft.com> - 2.28-10
- Added provides for binary capability.

* Thu Jun 11 2020 Henry Beberman <henry.beberman@microsoft.com> - 2.28-9
- Disable -Wp,-D_FORTIFY_SOURCE=2 to build with hardened cflags.

* Tue May 19 2020 Emre Girgin <mrgirgin@microsoft.com> - 2.28-8
- Ignore CVE-2019-6488, CVE-2020-1751, CVE-2020-6096 as they don't apply to aarch64 or x86_64.

* Sat May 09 2020 Nick Samson <nisamson@microsoft.com> - 2.28-7
- Added %%license line automatically

* Fri Mar 20 2020 Andrew Phelps <anphel@microsoft.com> - 2.28-6
- Configure with --disable-werror.

* Mon Dec 02 2019 Saravanan Somasundaram <sarsoma@microsoft.com> - 2.28-5
- Initial CBL-Mariner import from Photon (license: Apache2).

* Fri Jul 12 2019 Ankit Jain <ankitja@vmware.com> - 2.28-4
- Replaced spaces with tab in nsswitch.conf file

* Fri Mar 08 2019 Alexey Makhalov <amakhalov@vmware.com> - 2.28-3
- Fix CVE-2019-9169

* Tue Jan 22 2019 Anish Swaminathan <anishs@vmware.com> - 2.28-2
- Fix CVE-2018-19591

* Tue Aug 28 2018 Alexey Makhalov <amakhalov@vmware.com> - 2.28-1
- Version update. Disable obsolete rpc (use libtirpc) and nsl.

* Tue Jan 23 2018 Xiaolin Li <xiaolinl@vmware.com> - 2.26-10
- Fix CVE-2018-1000001 and CVE-2018-6485

* Mon Jan 08 2018 Xiaolin Li <xiaolinl@vmware.com> - 2.26-9
- Fix CVE-2017-16997

* Thu Dec 21 2017 Xiaolin Li <xiaolinl@vmware.com> - 2.26-8
- Fix CVE-2017-17426

* Tue Nov 14 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.26-7
- Aarch64 support

* Wed Oct 25 2017 Xiaolin Li <xiaolinl@vmware.com> - 2.26-6
- Fix CVE-2017-15670 and CVE-2017-15804

* Tue Oct 10 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.26-5
- Compile out tcache.

* Fri Sep 15 2017 Bo Gan <ganb@vmware.com> - 2.26-4
- exclude tst-eintr1 per official wiki recommendation.

* Tue Sep 12 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.26-3
- Fix makecheck for run in docker.

* Tue Aug 29 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.26-2
- Fix tunables setter.
- Add malloc arena fix.
- Fix makecheck.

* Tue Aug 15 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.26-1
- Version update

* Tue Aug 08 2017 Anish Swaminathan <anishs@vmware.com> - 2.25-4
- Apply fix for CVE-2017-1000366

* Thu May 4  2017 Bo Gan <ganb@vmware.com> - 2.25-3
- Remove bash dependency in post/postun script

* Fri Apr 21 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.25-2
- Added -iconv -tools and -nscd subpackages

* Wed Mar 22 2017 Alexey Makhalov <amakhalov@vmware.com> - 2.25-1
- Version update

* Wed Dec 14 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.24-1
- Version update

* Wed Nov 23 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.22-13
- Install en_US.UTF-8 locale by default

* Wed Nov 16 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.22-12
- Added i18n subpackage

* Tue Oct 25 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.22-11
- Workaround for build failure with "out of memory" message

* Wed Sep 28 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.22-10
- Added pthread_create-fix-use-after-free.patch

* Tue Jun 14 2016 Divya Thaluru <dthaluru@vmware.com> - 2.22-9
- Enabling rpm debug package and stripping the libraries

* Tue May 24 2016 Priyesh Padmavilasom <ppadmavilasom@vmware.com> - 2.22-8
- GA - Bump release of all rpms

* Mon May 23 2016 Divya Thaluru <dthaluru@vmware.com> - 2.22-7
- Added patch for CVE-2014-9761

* Mon Mar 21 2016 Alexey Makhalov <amakhalov@vmware.com> - 2.22-6
- Security hardening: nonow

* Fri Mar 18 2016 Anish Swaminathan <anishs@vmware.com> - 2.22-5
- Change conf file qualifiers

* Fri Mar 11 2016 Priyesh Padmavilasom <ppadmavilasom@vmware.com> - 2.22-4
- Added patch for res_qeury assertion with bad dns config
- Details: https://sourceware.org/bugzilla/show_bug.cgi?id=19791

* Tue Feb 16 2016 Anish Swaminathan <anishs@vmware.com> - 2.22-3
- Added patch for CVE-2015-7547

* Mon Feb 08 2016 Anish Swaminathan <anishs@vmware.com> - 2.22-2
- Added patch for bindresvport blacklist

* Tue Jan 12 2016 Xiaolin Li <xiaolinl@vmware.com> - 2.22-1
- Updated to version 2.22

* Tue Dec 1 2015 Divya Thaluru <dthaluru@vmware.com> - 2.19-8
- Disabling rpm debug package and stripping the libraries

* Wed Nov 18 2015 Divya Thaluru <dthaluru@vmware.com> - 2.19-7
- Adding patch to close nss files database

* Tue Nov 10 2015 Xiaolin Li <xiaolinl@vmware.com> - 2.19-6
- Handled locale files with macro find_lang

* Wed Aug 05 2015 Kumar Kaushik <kaushikk@vmware.com> - 2.19-5
- Adding postun section for ldconfig.

* Tue Jul 28 2015 Alexey Makhalov <amakhalov@vmware.com> - 2.19-4
- Support glibc building against current rpm version.

* Thu Jul 23 2015 Divya Thaluru <dthaluru@vmware.com> - 2.19-3
- Packing locale-gen scripts

* Mon May 18 2015 Touseef Liaqat <tliaqat@vmware.com> - 2.19-2
- Update according to UsrMove.

* Wed Nov 5 2014 Divya Thaluru <dthaluru@vmware.com> - 2.19-1
- Initial build. First version
