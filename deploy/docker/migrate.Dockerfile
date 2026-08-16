FROM mysql:8.4

ARG APPFORGE_SCHEMA_TARGET=20260815_113_v7_air_gapped

COPY services/system/system.sql /bootstrap/system.sql
COPY services/core/core.sql /bootstrap/core.sql
COPY deploy/mysql/init/30-seed.sql /bootstrap/seed.sql
COPY deploy/mysql/migrations /migrations
COPY deploy/docker/migrate.sh /usr/local/bin/appforge-migrate

RUN case "$APPFORGE_SCHEMA_TARGET" in \
      20260815_112_v7_customer_storage) \
        sed -i '/^-- APPFORGE_SCHEMA_113_BEGIN：/,/^-- APPFORGE_SCHEMA_113_END$/d' /bootstrap/core.sql; \
        rm -f /migrations/113-v7-air-gapped.sql; \
        ! grep -q 't_air_gapped_package' /bootstrap/core.sql; \
        test ! -e /migrations/113-v7-air-gapped.sql ;; \
      20260815_113_v7_air_gapped) \
        grep -q 't_air_gapped_package' /bootstrap/core.sql; \
        test -s /migrations/113-v7-air-gapped.sql ;; \
      *) \
        echo "不支持的迁移镜像Schema目标: $APPFORGE_SCHEMA_TARGET" >&2; \
        exit 1 ;; \
    esac \
    && chmod 0555 /usr/local/bin/appforge-migrate
ENTRYPOINT ["/usr/local/bin/appforge-migrate"]
