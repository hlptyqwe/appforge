FROM mysql:8.4 AS mysql-trust

FROM oraclelinux:9-slim

COPY --from=mysql-trust /etc/pki/rpm-gpg/RPM-GPG-KEY-mysql /etc/pki/rpm-gpg/RPM-GPG-KEY-mysql
RUN printf '%s\n' \
      '[mysql-8.4-lts-community]' \
      'name=MySQL 8.4 LTS Community Server' \
      'baseurl=https://repo.mysql.com/yum/mysql-8.4-community/el/9/$basearch/' \
      'enabled=1' \
      'gpgcheck=1' \
      'gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-mysql' \
      'module_hotfixes=true' \
      >/etc/yum.repos.d/mysql-8.4-community.repo \
    && microdnf install -y mysql-community-client \
    && microdnf clean all \
    && mysqlbinlog --version \
    && mysql --version

USER 65532:65532
ENTRYPOINT ["/bin/sh"]
