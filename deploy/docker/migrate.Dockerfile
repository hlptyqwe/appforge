FROM mysql:8.4

COPY services/system/system.sql /bootstrap/system.sql
COPY services/core/core.sql /bootstrap/core.sql
COPY deploy/mysql/init/30-seed.sql /bootstrap/seed.sql
COPY deploy/mysql/migrations /migrations
COPY deploy/docker/migrate.sh /usr/local/bin/appforge-migrate

RUN chmod 0555 /usr/local/bin/appforge-migrate
ENTRYPOINT ["/usr/local/bin/appforge-migrate"]
