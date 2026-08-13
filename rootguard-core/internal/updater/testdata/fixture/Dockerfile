# Minimal stand-in "service" for the pre-update snapshot/rollback
# integration test (see ../rollback_integration_test.go). Not a RootGuard
# component image - only the mechanics Manager itself depends on: a real
# container with a real BackupPaths directory and a real HEALTHCHECK.
FROM alpine:3.23

ARG HEALTHY=true
RUN mkdir -p /data && printf 'authentic data' > /data/marker \
    && if [ "$HEALTHY" = "true" ]; then touch /healthy; fi

HEALTHCHECK --interval=1s --timeout=1s --retries=1 --start-period=1s \
    CMD test -f /healthy

ENTRYPOINT ["sleep", "infinity"]
