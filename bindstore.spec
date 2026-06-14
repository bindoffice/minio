%define         tag     RELEASE.2020-11-25T22-36-25Z
%define         subver  %(echo %{tag} | sed -e 's/[^0-9]//g')
# git fetch https://github.com/bindoffice/bindstore.git refs/tags/RELEASE.2020-11-25T22-36-25Z
# git rev-list -n 1 FETCH_HEAD
%define         commitid        91130e884b5df59d66a45a0aad4f48db88f5ca63
Summary:        High Performance, Kubernetes Native Object Storage.
Name:           bindstore
Version:        0.0.%{subver}
Release:        1
Vendor:         BindOffice
License:        Apache v2.0
Group:          Applications/File
Source0:        https://github.com/bindoffice/bindstore/releases/download/%{tag}/bindstore.%{tag}
Source1:        https://raw.githubusercontent.com/minio/minio-service/master/linux-systemd/distributed/minio.service
URL:            https://github.com/bindoffice/bindstore
Requires(pre):  /usr/sbin/useradd, /usr/bin/getent
Requires(postun): /usr/sbin/userdel
BuildRoot:      %{tmpdir}/%{name}-%{version}-root-%(id -u -n)

## Disable debug packages.
%define         debug_package %{nil}

%description
bindstore is a High Performance Object Storage released under Apache License v2.0.
It is API compatible with Amazon S3 cloud storage service.

%pre
/usr/bin/getent group bindstore-user || /usr/sbin/groupadd -r bindstore-user
/usr/bin/getent passwd bindstore-user || /usr/sbin/useradd -r -d /etc/bindstore -s /sbin/nologin bindstore-user

%install
rm -rf $RPM_BUILD_ROOT
install -d $RPM_BUILD_ROOT/etc/bindstore/certs
install -d $RPM_BUILD_ROOT/etc/systemd/system
install -d $RPM_BUILD_ROOT/etc/default
install -d $RPM_BUILD_ROOT/usr/local/bin

cat <<EOF >> $RPM_BUILD_ROOT/etc/default/bindstore
# Remote volumes to be used for bindstore server.
# Uncomment line before starting the server.
# MINIO_VOLUMES=http://node{1...6}/export{1...32}

# Root credentials for the server.
# Uncomment both lines before starting the server.
# MINIO_ROOT_USER=Server-Root-User
# MINIO_ROOT_PASSWORD=Server-Root-Password

MINIO_OPTS="--certs-dir /etc/bindstore/certs"
EOF

install %{_sourcedir}/minio.service $RPM_BUILD_ROOT/etc/systemd/system/bindstore.service
install -p %{_sourcedir}/%{name}.%{tag} $RPM_BUILD_ROOT/usr/local/bin/bindstore

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(644,root,root,755)
%attr(644,root,root) /etc/default/bindstore
%attr(644,root,root) /etc/systemd/system/bindstore.service
%attr(644,bindstore-user,bindstore-user) /etc/bindstore
%attr(755,bindstore-user,bindstore-user) /usr/local/bin/bindstore
