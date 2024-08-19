Summary:        Test Package
Name:           test_pkg
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Azure Linux
Group:          Test
URL:            https://www.microsoft.com
Source0:        test.txt
Patch0:         test.patch
BuildRequires:  b >= 2.0

%description
Test package

%generate_buildrequires
echo dynamic_br < 3.0

%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "test" > %{buildroot}%{_sysconfdir}/TestPackages/test.txt

%check
echo "Testing"

%files
%{_sysconfdir}/TestPackages/test.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial Test
