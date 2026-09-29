ARG SOURCE_IMAGE=scratch
FROM ${SOURCE_IMAGE} AS source

FROM source
ARG PROFILE_FILE
COPY ${PROFILE_FILE} /boxen/profile.yaml
COPY --from=source /boxen/profile.yaml /tmp/boxen-source-profile.yaml
RUN version_line="$(sed -n '/^resolvedVersion:/p' /tmp/boxen-source-profile.yaml)" && \
    if [ -n "${version_line}" ] && ! grep -q '^resolvedVersion:' /boxen/profile.yaml; then \
        printf '\n%s\n' "${version_line}" >> /boxen/profile.yaml; \
    fi && \
    rm /tmp/boxen-source-profile.yaml
