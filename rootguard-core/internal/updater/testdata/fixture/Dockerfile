# Minimal stand-in "service" for the pre-update snapshot/rollback
# integration test (see ../rollback_integration_test.go). Not a RootGuard
# component image - only the mechanics Manager itself depends on: a real
# container with a real BackupPaths directory and a real HEALTHCHECK.
#
# Pinned by digest, not just the "3.23" tag - see dnssec-test-zone/setup.sh's
# identical comment on why. Found in review, round 12.
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG HEALTHY=true
# Two otherwise-identical healthy tags must still build to distinct image
# IDs, or Manager's candidate-vs-current comparison treats a "recovery"
# update as a same-ID no-op instead of a real swap. VARIANT only affects a
# label, never runtime behavior.
ARG VARIANT=default
LABEL io.rootguard.fixture.variant="${VARIANT}"
RUN adduser -D -u 10001 fixture \
    && mkdir -p /data && printf 'authentic data' > /data/marker && chown -R fixture:fixture /data \
    && if [ "$HEALTHY" = "true" ]; then touch /healthy && chown fixture:fixture /healthy; fi

HEALTHCHECK --interval=1s --timeout=1s --retries=1 --start-period=1s \
    CMD test -f /healthy

USER fixture
ENTRYPOINT ["sleep", "infinity"]
