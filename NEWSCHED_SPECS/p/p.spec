Summary:        Test Package p
Name:           p
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        p.txt
BuildRequires: d

%description
Test package p

%generate_buildrequires


%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "p" > %{buildroot}%{_sysconfdir}/TestPackages/p.txt

%check
echo "Testing p"

%files
%{_sysconfdir}/TestPackages/p.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
