Summary:        Test Package b
Name:           b
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        b.txt
BuildRequires: p

%description
Test package b

%generate_buildrequires
[ -f %{_sysconfdir}/TestPackages/p.txt ] || exit 1 ; echo c

%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "b" > %{buildroot}%{_sysconfdir}/TestPackages/b.txt

%check
echo "Testing b"

%files
%{_sysconfdir}/TestPackages/b.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
