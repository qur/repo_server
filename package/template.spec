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
if [ $1 -eq 1 ]; then
    # New Install
    /usr/sbin/useradd -c "Repo Server" -d @@datadir@@ -m -r -U repo
else
    # Upgrade
    /sbin/service @@name@@ stop
fi


%post
if [ $1 -eq 1 ]; then
    # New Install
    /sbin/chkconfig --add @@name@@
    /sbin/chkconfig @@name@@ on
fi
/sbin/service @@name@@ start


%preun
if [ $1 -eq 0 ]; then
    # Uninstall
    /sbin/service @@name@@ stop
    /sbin/chkconfig @@name@@ off
    /sbin/chkconfig --del @@name@@
fi


%postun
if [ $1 -eq 0 ]; then
    /usr/sbin/userdel repo
fi


%changelog
* Fri Dec 2 2016 Stuart Websper
- Fixed calculation of SHA256 / MD5SUM for Packages.gz
