# Render Results (April 9, 2026)

**7557** ok, **45** failed out of 7602 components.

## Autorelease — should already be handled (6)

These use `%{autorelease}` which the tool claims to support. Needs investigation.

| Component | Release value |
|-----------|---------------|
| 389-ds-base | `%{autorelease -n %{?with_asan:-e asan}}%{?dist}` |
| gnutls | `%{?autorelease}%{!?autorelease:1%{?dist}}` |
| keylime-agent-rust | `%{?autorelease}%{!?autorelease:1%{?dist}}` |
| libunistring | `%{?autorelease}%{!?autorelease:1%{?dist}}` |
| nettle | `%{?autorelease}%{!?autorelease:1%{?dist}}` |
| p11-kit | `%{?autorelease}%{!?autorelease:1%{?dist}}` |

## Release tag not auto-bumpable (37)

These specs use macro-computed `Release:` tags that don't start with an integer. Each needs a `spec-set-tag` overlay for the Release tag, or the tool needs to learn these patterns.

| Component | Release value |
|-----------|---------------|
| antlr3 | `%{baserelease}%{?dist}` |
| cmake | `%{baserelease}%{?dist}` |
| cpuinfo | `%{patch_level}.git%{?shortcommit0}%{?dist}.2` |
| cross-gcc | `%{cross_gcc_release}%{?dist}` |
| dogtag-pki | `%{release_number}%{?phase:.}%{?phase}%{?timestamp:.}%{?timestamp}%{?commit_id:.}%{?commit_id}%{?dist}` |
| elfutils | `%{baserelease}%{?dist}` |
| gcc | `%{gcc_release}%{?dist}` |
| glibc | `%{baserelease}%{?dist}` |
| java-21-openjdk | `%{?eaprefix}%{rpmrelease}%{?extraver}%{?dist}.1` |
| java-25-openjdk | `%{?eaprefix}%{rpmrelease}%{?extraver}%{?dist}` |
| java-25-openjdk-portable | `%{?eaprefix}%{rpmrelease}%{?extraver}%{?dist}` |
| jss | `%{release_number}%{?phase:.}%{?phase}%{?timestamp:.}%{?timestamp}%{?commit_id:.}%{?commit_id}%{?dist}` |
| kernel | `%{pkg_release}` |
| kernel-headers | `%{specrelease}` |
| krb5 | `%{krb5_release}` |
| ldapjdk | `%{release_number}%{?phase:.}%{?phase}%{?timestamp:.}%{?timestamp}%{?commit_id:.}%{?commit_id}%{?dist}` |
| mod_proxy_cluster | `%{serial}%{?dist}.1` |
| nodejs22 | `%{nodejs_release}` |
| nodejs24 | `%{node_release}` |
| nss | `%{nss_release}%{?dist}` |
| oniguruma | `%{?prerelease:0.}%{baserelease}%{?dist}` |
| osbs-client | `%{release}%{?dist}` |
| pacemaker | `%{pcmk_release}%{?dist}` |
| pcre | `%{?rcversion:0.}1%{?rcversion:.%rcversion}%{?dist}.9` |
| pcre2 | `%{?rcversion:0.}1%{?rcversion:.%rcversion}%{?dist}` |
| pipewire | `%{baserelease}%{?snapdate:.%{snapdate}git%{shortcommit}}%{?dist}` |
| rpm | `%{?snapver:0.%{snapver}.}%{baserelease}%{?dist}` |
| rubygem-nokogiri | `%{?prever:0.}%{baserelease}%{?prever:.%{prerpmver}}%{?dist}` |
| rubygem-rake | `%{?preminorver:0.}%{baserelease}%{?preminorver:%{rpmminorver}}%{?dist}` |
| rubygem-rspec-core | `%{?preminorver:0.}%{baserelease}%{?preminorver:%{rpmminorver}}%{?dist}` |
| rubygem-rspec-expectations | `%{?preminorver:0.}%{baserelease}%{?preminorver:%{rpmminorver}}%{?dist}` |
| rubygem-rspec-mocks | `%{?preminorver:0.}%{baserelease}%{?preminorver:%{rpmminorver}}%{?dist}` |
| rubygem-rspec-support | `%{?prever:0.}%{baserelease}%{?prever:.%{prerpmver}}%{?dist}` |
| samba | `%{samba_release}` |
| sbd | `%{baserelease}%{?dist}` |
| softhsm | `%{?prever:0.}13%{?prever:.%{prever}}%{?dist}.1` |
| xhtml1-dtds | `%{date}.%{baserelease}%{?dist}` |

## Spectool parse error (1)

| Component | Error |
|-----------|-------|
| glusterfs | `spectool failed: error: line 203: Illegal char '@' (0x40) in: Release: 1.@PACKAGE_RELEASE@.azl4.26` |

## Staging directory error (1)

| Component | Error |
|-----------|-------|
| at | `read /tmp/azldev-render-staging-490374529/at/tests/at: is a directory` |

