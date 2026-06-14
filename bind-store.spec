%define         tag     RELEASE.2020-11-25T22-36-25Z
%define         subver  %(echo %{tag} | sed -e 's/[^0-9]//g')
# git fetch https://github.com/bindoffice/bind-store.git refs/tags/RELEASE.2020-11-25T22-36-25Z
# git rev-list -n 1 FETCH_HEAD
%define         commitid        91130e884b5df59d66a45a0aad4f48db88f5ca63
Summary:        High Performance, Kubernetes Native Object Storage.
Name:           bind-store
Version:        0.0.%{subver}
Release:        1
Vendor:         BindOffice
License:        Apache v2.0
Group:          Applications/File
Source0:        https://github.com/bindoffice/bind-store/releases/download/%{tag}/bind-store.%{tag}
Source1:        https://raw.githubusercontent.com/minio/minio-service/master/linux-systemd/distributed/minio.service
URL:            https://github.com/bindoffice/bind-store
Requires(pre):  /usr/sbin/useradd, /usr/bin/getent
Requires(postun): /usr/sbin/userdel
BuildRoot:      %{tmpdir}/%{name}-%{version}-root-%(id -u -n)

## Disable debug packages.
%define         debug_package %{nil}

%description
bind-store is a High Performance Object Storage released under Apache License v2.0.
It is API compatible with Amazon S3 cloud storage service.

%pre
/usr/bin/getent group bind-store-user || /usr/sbin/groupadd -r bind-store-user
/usr/bin/getent passwd bind-store-user || /usr/sbin/useradd -r -d /etc/bind-store -s /sbin/nologin bind-store-user

%install
rm -rf $RPM_BUILD_ROOT
install -d $RPM_BUILD_ROOT/etc/bind-store/certs
install -d $RPM_BUILD_ROOT/etc/systemd/system
install -d $RPM_BUILD_ROOT/etc/default
install -d $RPM_BUILD_ROOT/usr/local/bin

cat <<EOF >> $RPM_BUILD_ROOT/etc/default/bind-store
# Remote volumes to be used for bind-store server.
# Uncomment line before starting the server.
# MINIO_VOLUMES=http://node{1...6}/export{1...32}

# Root credentials for the server.
# Uncomment both lines before starting the server.
# MINIO_ROOT_USER=Server-Root-User
# MINIO_ROOT_PASSWORD=Server-Root-Password

MINIO_OPTS="--certs-dir /etc/bind-store/certs"
EOF

install %{_sourcedir}/minio.service $RPM_BUILD_ROOT/etc/systemd/system/bind-store.service
install -p %{_sourcedir}/%{name}.%{tag} $RPM_BUILD_ROOT/usr/local/bin/bind-store

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(644,root,root,755)
%attr(644,root,root) /etc/default/bind-store
%attr(644,root,root) /etc/systemd/system/bind-store.service
%attr(644,bind-store-user,bind-store-user) /etc/bind-store
%attr(755,bind-store-user,bind-store-user) /usr/local/bin/bind-store
