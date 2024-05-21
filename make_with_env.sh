#!/bin/sh
# make_with_env.sh

# Source the .env file
if [ -f .env ]; then
    . ./.env
fi

# Pass all environment variables to make
exec make "$@" GREENLIGHT_DB_DSN="$GREENLIGHT_DB_DSN"
