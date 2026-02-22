#!/usr/bin/env sh
# Fetch official Filestash Dockerfile, inject plugin, build.
# Use --generate-only (or -g) to only refresh Dockerfile from upstream (no build).

set -e
cd "$(dirname "$0")"
GENERATE_ONLY=
[ "$1" = "--generate-only" ] || [ "$1" = "-g" ] && GENERATE_ONLY=1

URL="${FILESTASH_DOCKERFILE_URL:-https://raw.githubusercontent.com/mickael-kerjean/filestash/master/docker/Dockerfile}"

curl -sSL "$URL" | awk '
  /^COPY --from=builder_prepare \/home\/filestash\/ \.$/ {
    print
    print ""
    print "COPY server/plugin/plg_authenticate_proxy_header/ server/plugin/plg_authenticate_proxy_header/"
    print "RUN sed -i '\''/plg_authenticate_passthrough\"/a\\\t_ \"github.com/mickael-kerjean/filestash/server/plugin/plg_authenticate_proxy_header\"'\'' server/plugin/index.go"
    print ""
    next
  }
  { print }
' > Dockerfile

[ -n "$GENERATE_ONLY" ] && echo "Refreshed Dockerfile from upstream. Run docker-compose build or ./build.sh to build." && exit 0

docker build -t filestash-with-proxy-auth -f Dockerfile .
