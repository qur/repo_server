Name:           @@name@@
Version:        @@version@@
Release:        1
Summary:        A Dynamic Ubuntu Repository Server

Group:          Applications/Internet
License:        BSD
URL:            http://notyet.example.com
Source0:        @@name@@-@@version@@.tar.gz

#BuildRequires:  
#Requires:       

%define debug_package %{nil}

%description


%prep
%setup -q


%build
/bin/true


%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/usr/bin
mkdir -p $RPM_BUILD_ROOT/etc/init.d
mkdir -p $RPM_BUILD_ROOT/etc/sysconfig
mkdir -p $RPM_BUILD_ROOT/@@datadir@@
cp @@name@@ $RPM_BUILD_ROOT/usr/bin/
cp @@name@@.init $RPM_BUILD_ROOT/etc/init.d/@@name@@
cp sysconfig $RPM_BUILD_ROOT/etc/sysconfig/@@name@@
cp config.yml $RPM_BUILD_ROOT/@@datadir@@
cp keyring $RPM_BUILD_ROOT/@@datadir@@


%clean
rm -rf $RPM_BUILD_ROOT


%files
%defattr(-,root,root,-)
/etc/init.d/@@name@@
/etc/sysconfig/@@name@@
/usr/bin/@@name@@
%defattr(-,repo,repo,-)
@@datadir@@
%doc


%pre
/bin/mkdir -p "$(dirname "@@datadir@@")"
/usr/sbin/useradd -c "Repo Server" -d @@datadir@@ -m -r -U repo


%post
/sbin/chkconfig --add @@name@@
/sbin/chkconfig @@name@@ on
/sbin/service @@name@@ start


%preun
/sbin/service @@name@@ stop
/sbin/chkconfig @@name@@ off
/sbin/chkconfig --del @@name@@


%postun
/usr/sbin/userdel repo


%changelog
