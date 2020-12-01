Summary:        Verity testing
Name:           verity-readonly-root
Version:        1.1
Release:        30%{?dist}
License:        Charityware
URL:            http://www.vim.org
Group:          Applications/Editors
Vendor:         Microsoft Corporation
Distribution:   Mariner
Source0:        init_verity.sh
Source1:        verity.conf
Source2:        mount_verity.sh
Source3:        20verity-mount/module-setup.sh
Source4:        20verity-mount/verity-root-generator.sh
Source5:        20verity-mount/verity_parse_v2.sh
Source6:        20verity-mount/verity_mount_v2.sh
Requires:       veritysetup
Requires:       device-mapper
Requires:       dracut

%description
vt

%install
mkdir -p %{buildroot}
cp %{SOURCE0} %{buildroot}/init_verity.sh
cp %{SOURCE2} %{buildroot}/mount_verity.sh
mkdir -p %{buildroot}%{_sysconfdir}/dracut.conf.d
install -D -m644 %{SOURCE1} %{buildroot}%{_sysconfdir}/dracut.conf.d/
mkdir -p %{buildroot}%{_libdir}/dracut/modules.d/20verity-mount/
cp %{SOURCE3} %{buildroot}%{_libdir}/dracut/modules.d/20verity-mount/
# cp %{SOURCE4} %{buildroot}%{_libdir}/dracut/modules.d/20verity-mount/
cp %{SOURCE5} %{buildroot}%{_libdir}/dracut/modules.d/20verity-mount/
cp %{SOURCE6} %{buildroot}%{_libdir}/dracut/modules.d/20verity-mount/
mkdir -p %{buildroot}/overlay_tmpfs_mnt

%files
/init_verity.sh
/mount_verity.sh
%{_sysconfdir}/dracut.conf.d/verity.conf
%dir %{_libdir}/dracut/modules.d/20verity-mount
%{_libdir}/dracut/modules.d/20verity-mount/*
%dir /overlay_tmpfs_mnt

%changelog
*   Wed Nov 13 2019 Dan <damcilva@microsoft.com> 1.1-30
-   Initial Mariner version 2.
*   Wed Nov 13 2019 Dan <damcilva@microsoft.com> 1.0
-   Initial Mariner version.