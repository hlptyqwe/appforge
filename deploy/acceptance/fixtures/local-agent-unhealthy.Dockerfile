ARG BASE_IMAGE
FROM ${BASE_IMAGE}
USER root
COPY --chmod=0755 local-agent-unhealthy-entrypoint.sh /usr/local/bin/acceptance-entrypoint
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/acceptance-entrypoint"]
