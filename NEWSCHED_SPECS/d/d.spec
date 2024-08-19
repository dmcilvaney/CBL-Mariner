Summary:        Test Package d
Name:           d
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        d.txt
BuildRequires: c

%description
Test package d

%generate_buildrequires


%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "d" > %{buildroot}%{_sysconfdir}/TestPackages/d.txt

%check
echo "Testing d"

%files
%{_sysconfdir}/TestPackages/d.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
