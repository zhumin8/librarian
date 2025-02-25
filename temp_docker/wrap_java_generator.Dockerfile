FROM local:image-tag AS base

# Create a new image based on the existing one
FROM base AS wrapper

# Set the working directory (optional, but can be helpful)
WORKDIR /workspace

# hardcode version to test
ENV GENERATOR_VERSION 2.53.1-SNAPSHOT

# Define the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
