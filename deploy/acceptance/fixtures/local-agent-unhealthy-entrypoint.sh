#!/bin/sh

if [ "${1:-}" = version ]; then
  exec /usr/local/bin/appforge-local-agent "$@"
fi
exit 17
