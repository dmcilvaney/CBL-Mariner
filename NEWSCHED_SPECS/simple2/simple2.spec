Summary:        Test Package simple2
Name:           simple2
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        simple2.txt

Provides: %{name}_other_thing = %{version}

%description
Test package simple2

%generate_buildrequires


%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "simple2" > %{buildroot}%{_sysconfdir}/TestPackages/simple2.txt

%check
echo "Testing simple2"

%files
%{_sysconfdir}/TestPackages/simple2.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
