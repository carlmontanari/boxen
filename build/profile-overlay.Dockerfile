ARG SOURCE_IMAGE=scratch
ARG BOXEN_IMAGE=scratch
FROM ${SOURCE_IMAGE} AS source
FROM ${BOXEN_IMAGE} AS boxen_runtime

FROM source
ARG PROFILE_FILE
COPY --from=boxen_runtime /boxen/boxen /boxen/boxen
COPY --from=boxen_runtime /boxen/.libscrapli.so /boxen/.libscrapli.so
COPY --from=boxen_runtime /boxen/.scrapligo_definition.yaml /boxen/.scrapligo_definition.yaml
ENV LIBSCRAPLI_PATH=/boxen/.libscrapli.so
COPY ${PROFILE_FILE} /boxen/profile.yaml
COPY --from=source /boxen/profile.yaml /tmp/boxen-source-profile.yaml
RUN version_line="$(sed -n '/^resolvedVersion:/p' /tmp/boxen-source-profile.yaml)" && \
    if [ -n "${version_line}" ] && ! grep -q '^resolvedVersion:' /boxen/profile.yaml; then \
        printf '\n%s\n' "${version_line}" >> /boxen/profile.yaml; \
    fi && \
    rm /tmp/boxen-source-profile.yaml
