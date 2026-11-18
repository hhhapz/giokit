#!/bin/sh
# Builds the gallery and uploads the site to giokit.hamza.sh.
set -e
cd "$(dirname "$0")"
./build.sh
rsync -a --delete-excluded --exclude build.sh --exclude deploy.sh --exclude .gitignore ./ altair:/srv/static/giokit.hamza.sh/
