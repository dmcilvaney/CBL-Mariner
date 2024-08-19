Summary:        Test Package c
Name:           c
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
BuildRequires:  c
Requires:       simple2
BuildRequires:  words
Source0:        c.txt

%description
Test package c

%generate_buildrequires

%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
cat %{SOURCE0} > %{buildroot}%{_sysconfdir}/TestPackages/c.txt

%check
echo "Testing c"

%files
%{_sysconfdir}/TestPackages/c.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
