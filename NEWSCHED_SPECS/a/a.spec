Summary:        Test Package a
Name:           a
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        a.txt
BuildRequires: p d

%description
Test package a

%generate_buildrequires
[ -f %{_sysconfdir}/TestPackages/p.txt ] || exit 1 ; echo b

%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "a" > %{buildroot}%{_sysconfdir}/TestPackages/a.txt

%check
echo "Testing a"

%files
%{_sysconfdir}/TestPackages/a.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
