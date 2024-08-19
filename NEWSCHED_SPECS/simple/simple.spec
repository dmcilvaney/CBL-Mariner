Summary:        Test Package simple
Name:           simple
Version:        1
Release:        1%{?dist}
License:        MIT
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          Applications/Editors
URL:            https://www.microsoft.com
Source0:        simple.txt
Requires: simple2
BuildRequires: simple2

Provides: %{name}_other_thing = %{version}

%description
Test package simple

%generate_buildrequires


%build
mkdir -p %{buildroot}%{_sysconfdir}/TestPackages/
echo "simple" > %{buildroot}%{_sysconfdir}/TestPackages/simple.txt

%check
echo "Testing simple"

%files
%{_sysconfdir}/TestPackages/simple.txt

%changelog
* Tue Aug 22 2023 Daniel McIlvaney <damcilva@microsoft.com> - 1-1
- Initial
