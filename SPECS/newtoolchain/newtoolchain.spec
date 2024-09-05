%bcond debug 1

Summary:        azl-toolchain
Name:           azl-toolchain
Version:        1
Release:        1%{?dist}
License:        MIT and GPLv2 and GPLv2+ and BSD
URL:            https://github.com/microsoft/azurelinux
Group:          Applications/Nfs-utils-client
Vendor:         Microsoft Corporation
Distribution:   Azure Linux

Requires: acl
Requires: asciidoc
Requires: attr
Requires: audit
Requires: audit-devel
Requires: audit-libs
Requires: autoconf
Requires: automake
Requires: azurelinux-check-macros
Requires: azurelinux-repos
Requires: azurelinux-repos-debug
Requires: azurelinux-repos-debug-preview
Requires: azurelinux-repos-extended
Requires: azurelinux-repos-extended-debug
Requires: azurelinux-repos-extended-debug-preview
Requires: azurelinux-repos-extended-preview
Requires: azurelinux-repos-ms-non-oss
Requires: azurelinux-repos-ms-non-oss-preview
Requires: azurelinux-repos-ms-oss
Requires: azurelinux-repos-ms-oss-preview
Requires: azurelinux-repos-preview
Requires: azurelinux-repos-shared
Requires: azurelinux-rpm-macros
Requires: bash
Requires: bash-devel
Requires: bash-lang
Requires: binutils
Requires: binutils-aarch64-linux-gnu
Requires: binutils-devel
Requires: bison
Requires: bzip2
Requires: bzip2-devel
Requires: bzip2-libs
Requires: ca-certificates
Requires: ca-certificates-base
Requires: ca-certificates-legacy
Requires: ca-certificates-shared
Requires: ca-certificates-tools
Requires: ccache
Requires: check
Requires: chkconfig
Requires: chkconfig-lang
Requires: cmake
Requires: coreutils
Requires: coreutils-lang
Requires: cpio
Requires: cpio-lang
Requires: cracklib
Requires: cracklib-devel
Requires: cracklib-dicts
Requires: cracklib-lang
Requires: createrepo_c
Requires: createrepo_c-devel
Requires: cross-binutils-common
Requires: cross-gcc-common
Requires: curl
Requires: curl-devel
Requires: curl-libs
Requires: debugedit
Requires: diffutils
Requires: docbook-dtd-xml
Requires: docbook-style-xsl
Requires: dwz
Requires: e2fsprogs
Requires: e2fsprogs-devel
Requires: e2fsprogs-lang
Requires: e2fsprogs-libs
Requires: elfutils
Requires: elfutils-default-yama-scope
Requires: elfutils-devel
Requires: elfutils-devel-static
Requires: elfutils-libelf
Requires: elfutils-libelf-devel
Requires: elfutils-libelf-devel-static
Requires: elfutils-libelf-lang
Requires: expat
Requires: expat-devel
Requires: expat-libs
Requires: file
Requires: file-devel
Requires: file-libs
Requires: filesystem
Requires: filesystem-asc
Requires: findutils
Requires: findutils-lang
Requires: flex
Requires: flex-devel
Requires: gawk
Requires: gcc
Requires: gcc-aarch64-linux-gnu
Requires: gcc-c++
Requires: gcc-c++-aarch64-linux-gnu
Requires: gdbm
Requires: gdbm-devel
Requires: gdbm-lang
Requires: gettext
Requires: gfortran
Requires: glib
Requires: glib-devel
Requires: glib-doc
Requires: glib-schemas
Requires: glibc
Requires: glibc-devel
Requires: glibc-i18n
Requires: glibc-iconv
Requires: glibc-lang
Requires: glibc-locales-all
Requires: glibc-nscd
Requires: glibc-static
Requires: glibc-tools
Requires: gmp
Requires: gmp-devel
Requires: gnupg2
Requires: gnupg2-lang
Requires: gperf
Requires: gpgme
Requires: gpgme-devel
Requires: grep
Requires: grep-lang
Requires: gtk-doc
Requires: gzip
Requires: intltool
Requires: itstool
Requires: kbd
Requires: kernel-cross-headers
Requires: kernel-headers
Requires: kmod
Requires: kmod-devel
Requires: krb5
Requires: krb5-devel
Requires: krb5-lang
Requires: libacl
Requires: libacl-devel
Requires: libarchive
Requires: libarchive-devel
Requires: libassuan
Requires: libassuan-devel
Requires: libattr
Requires: libattr-devel
Requires: libbacktrace-static
Requires: libcap
Requires: libcap-devel
Requires: libcap-ng
Requires: libcap-ng-devel
Requires: libffi
Requires: libffi-devel
Requires: libgcc
Requires: libgcc-atomic
Requires: libgcc-devel
Requires: libgcrypt
Requires: libgcrypt-devel
Requires: libgomp
Requires: libgomp-devel
Requires: libgpg-error
Requires: libgpg-error-devel
Requires: libgpg-error-lang
Requires: libksba
Requires: libksba-devel
Requires: libltdl
Requires: libltdl-devel
Requires: libmetalink
Requires: libmetalink-devel
Requires: libmpc
Requires: libpcre2-16-0
Requires: libpcre2-32-0
Requires: libpcre2-8-0
Requires: libpcre2-posix2
Requires: libpipeline
Requires: libpipeline-devel
Requires: libpkgconf
Requires: libpkgconf-devel
Requires: libselinux
Requires: libselinux-devel
Requires: libselinux-python3
Requires: libselinux-utils
Requires: libsepol
Requires: libsepol-devel
Requires: libsolv
Requires: libsolv-devel
Requires: libsolv-tools
Requires: libssh2
Requires: libssh2-devel
Requires: libstdc++
Requires: libstdc++-devel
Requires: libtasn1
Requires: libtasn1-devel
Requires: libtool
Requires: libxml2
Requires: libxml2-devel
Requires: libxcrypt
Requires: libxcrypt-devel
Requires: libxslt
Requires: libxslt-devel
Requires: lua
Requires: lua-devel
Requires: lua-libs
Requires: lua-rpm-macros
Requires: lua-srpm-macros
Requires: lua-static
Requires: lz4
Requires: lz4-devel
Requires: m4
Requires: make
Requires: meson
Requires: mpfr
Requires: mpfr-devel
Requires: msopenjdk-17
Requires: ncurses
Requires: ncurses-compat
Requires: ncurses-devel
Requires: ncurses-libs
Requires: ncurses-term
Requires: newt
Requires: newt-devel
Requires: newt-lang
Requires: nghttp2
Requires: nghttp2-devel
Requires: ninja-build
Requires: npth
Requires: npth-devel
Requires: ntsysv
Requires: ocaml-srpm-macros
Requires: openssl
Requires: openssl-devel
Requires: openssl-libs
Requires: openssl-perl
Requires: openssl-static
Requires: p11-kit
Requires: p11-kit-devel
Requires: p11-kit-server
Requires: p11-kit-trust
Requires: pam
Requires: pam-devel
Requires: pam-lang
Requires: patch
Requires: pcre2
Requires: pcre2-devel
Requires: pcre2-devel-static
Requires: pcre2-doc
Requires: pcre2-tools
Requires: perl
Requires: perl-Archive-Tar
Requires: perl-Attribute-Handlers
Requires: perl-autodie
Requires: perl-AutoLoader
Requires: perl-AutoSplit
Requires: perl-autouse
Requires: perl-B
Requires: perl-base
Requires: perl-Benchmark
Requires: perl-bignum
Requires: perl-blib
Requires: perl-Carp
Requires: perl-Class-Struct
Requires: perl-Compress-Raw-Bzip2
Requires: perl-Compress-Raw-Zlib
Requires: perl-Config-Extensions
Requires: perl-Config-Perl-V
Requires: perl-constant
Requires: perl-CPAN
Requires: perl-CPAN-Meta
Requires: perl-CPAN-Meta-Requirements
Requires: perl-CPAN-Meta-YAML
Requires: perl-Data-Dumper
Requires: perl-DBD-SQLite
Requires: perl-DBI
Requires: perl-DBIx-Simple
Requires: perl-DBM_Filter
Requires: perl-debugger
Requires: perl-deprecate
Requires: perl-devel
Requires: perl-Devel-Peek
Requires: perl-Devel-PPPort
Requires: perl-Devel-SelfStubber
Requires: perl-diagnostics
Requires: perl-Digest
Requires: perl-Digest-MD5
Requires: perl-Digest-SHA
Requires: perl-DirHandle
Requires: perl-doc
Requires: perl-Dumpvalue
Requires: perl-DynaLoader
Requires: perl-Encode
Requires: perl-Encode-devel
Requires: perl-encoding
Requires: perl-encoding-warnings
Requires: perl-English
Requires: perl-Env
Requires: perl-Errno
Requires: perl-experimental
Requires: perl-Exporter
Requires: perl-ExtUtils-CBuilder
Requires: perl-ExtUtils-Command
Requires: perl-ExtUtils-Constant
Requires: perl-ExtUtils-Embed
Requires: perl-ExtUtils-Install
Requires: perl-ExtUtils-MakeMaker
Requires: perl-ExtUtils-Manifest
Requires: perl-ExtUtils-Miniperl
Requires: perl-ExtUtils-MM-Utils
Requires: perl-ExtUtils-ParseXS
Requires: perl-Fcntl
Requires: perl-Fedora-VSP
Requires: perl-fields
Requires: perl-File-Basename
Requires: perl-File-Compare
Requires: perl-File-Copy
Requires: perl-File-DosGlob
Requires: perl-File-Fetch
Requires: perl-File-Find
Requires: perl-File-Path
Requires: perl-File-stat
Requires: perl-File-Temp
Requires: perl-FileCache
Requires: perl-FileHandle
Requires: perl-filetest
Requires: perl-Filter
Requires: perl-Filter-Simple
Requires: perl-FindBin
Requires: perl-GDBM_File
Requires: perl-generators
Requires: perl-Getopt-Long
Requires: perl-Getopt-Std
Requires: perl-Hash-Util
Requires: perl-Hash-Util-FieldHash
Requires: perl-HTTP-Tiny
Requires: perl-I18N-Collate
Requires: perl-I18N-Langinfo
Requires: perl-I18N-LangTags
Requires: perl-if
Requires: perl-interpreter
Requires: perl-IO
Requires: perl-IO-Compress
Requires: perl-IO-Socket-IP
Requires: perl-IO-Zlib
Requires: perl-IPC-Cmd
Requires: perl-IPC-Open3
Requires: perl-IPC-SysV
Requires: perl-JSON-PP
Requires: perl-less
Requires: perl-lib
Requires: perl-libintl-perl
Requires: perl-libnet
Requires: perl-libnetcfg
Requires: perl-libs
Requires: perl-locale
Requires: perl-Locale-Maketext
Requires: perl-Locale-Maketext-Simple
Requires: perl-macros
Requires: perl-Math-BigInt
Requires: perl-Math-BigInt-FastCalc
Requires: perl-Math-BigRat
Requires: perl-Math-Complex
Requires: perl-Memoize
Requires: perl-meta-notation
Requires: perl-MIME-Base64
Requires: perl-Module-CoreList
Requires: perl-Module-CoreList-tools
Requires: perl-Module-Load
Requires: perl-Module-Load-Conditional
Requires: perl-Module-Loaded
Requires: perl-Module-Metadata
Requires: perl-mro
Requires: perl-NDBM_File
Requires: perl-Net
Requires: perl-Net-Ping
Requires: perl-NEXT
Requires: perl-Object-Accessor
Requires: perl-ODBM_File
Requires: perl-Opcode
Requires: perl-open
Requires: perl-overload
Requires: perl-overloading
Requires: perl-Params-Check
Requires: perl-parent
Requires: perl-PathTools
Requires: perl-Perl-OSType
Requires: perl-perlfaq
Requires: perl-PerlIO-via-QuotedPrint
Requires: perl-ph
Requires: perl-Pod-Checker
Requires: perl-Pod-Escapes
Requires: perl-Pod-Functions
Requires: perl-Pod-Html
Requires: perl-Pod-Perldoc
Requires: perl-Pod-Simple
Requires: perl-Pod-Usage
Requires: perl-podlators
Requires: perl-POSIX
Requires: perl-Safe
Requires: perl-Scalar-List-Utils
Requires: perl-Search-Dict
Requires: perl-SelectSaver
Requires: perl-SelfLoader
Requires: perl-sigtrap
Requires: perl-Socket
Requires: perl-sort
Requires: perl-Storable
Requires: perl-subs
Requires: perl-Symbol
Requires: perl-Sys-Hostname
Requires: perl-Sys-Syslog
Requires: perl-Term-ANSIColor
Requires: perl-Term-Cap
Requires: perl-Term-Complete
Requires: perl-Term-ReadLine
Requires: perl-Test
Requires: perl-Test-Harness
Requires: perl-Test-Simple
Requires: perl-Test-Warnings
Requires: perl-tests
Requires: perl-Text-Abbrev
Requires: perl-Text-Balanced
Requires: perl-Text-ParseWords
Requires: perl-Text-Tabs+Wrap
Requires: perl-Text-Template
Requires: perl-Thread
Requires: perl-Thread-Queue
Requires: perl-Thread-Semaphore
Requires: perl-threads
Requires: perl-threads-shared
Requires: perl-Tie
Requires: perl-Tie-File
Requires: perl-Tie-Memoize
Requires: perl-Tie-RefHash
Requires: perl-Time
Requires: perl-Time-HiRes
Requires: perl-Time-Local
Requires: perl-Time-Piece
Requires: perl-Unicode-Collate
Requires: perl-Unicode-Normalize
Requires: perl-Unicode-UCD
Requires: perl-User-pwent
Requires: perl-utils
Requires: perl-vars
Requires: perl-version
Requires: perl-vmsish
Requires: perl-XML-Parser
Requires: pinentry
Requires: pkgconf
Requires: pkgconf-m4
Requires: pkgconf-pkg-config
Requires: popt
Requires: popt-devel
Requires: popt-lang
Requires: procps-ng
Requires: procps-ng-devel
Requires: procps-ng-lang
Requires: pyproject-rpm-macros
Requires: pyproject-srpm-macros
Requires: python-wheel-wheel
Requires: python3
Requires: python3-audit
Requires: python3-cracklib
Requires: python3-curses
Requires: python3-Cython
Requires: python3-devel
Requires: python3-flit-core
Requires: python3-gpg
Requires: python3-jinja2
Requires: python3-libcap-ng
Requires: python3-libs
Requires: python3-libxml2
Requires: python3-lxml
Requires: python3-magic
Requires: python3-markupsafe
Requires: python3-newt
Requires: python3-packaging
Requires: python3-pip
Requires: python3-pygments
Requires: python3-rpm
Requires: python3-rpm-generators
Requires: python3-setuptools
Requires: python3-test
Requires: python3-tools
Requires: python3-wheel
Requires: readline
Requires: readline-devel
Requires: rpm
Requires: rpm-build
Requires: rpm-build-libs
Requires: rpm-devel
Requires: rpm-lang
Requires: rpm-libs
Requires: sed
Requires: sed-lang
Requires: slang
Requires: slang-devel
Requires: sqlite
Requires: sqlite-devel
Requires: sqlite-libs
Requires: swig
Requires: systemd
Requires: systemd-devel
Requires: systemd-libs
Requires: systemd-rpm-macros
Requires: tar
Requires: tdnf
Requires: tdnf-autoupdate
Requires: tdnf-cli-libs
Requires: tdnf-devel
Requires: tdnf-plugin-metalink
Requires: tdnf-plugin-repogpgcheck
Requires: tdnf-python
Requires: texinfo
Requires: unzip
Requires: util-linux
Requires: util-linux-devel
Requires: util-linux-lang
Requires: util-linux-libs
Requires: which
Requires: xz
Requires: xz-devel
Requires: xz-lang
Requires: xz-libs
Requires: zip
Requires: zlib
Requires: zlib-devel
Requires: zstd
Requires: zstd-devel
Requires: zstd-doc
Requires: zstd-libs

%description
Meta package describing the Azure Linux toolchain

%prep

%build

%files
%defattr(-,root,root,0755)
