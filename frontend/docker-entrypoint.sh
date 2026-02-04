#!/bin/sh

# Recreate config.js with environment variables
echo "window.env = {" > /usr/share/nginx/html/config.js
echo "  VITE_API_URL: \"$VITE_API_URL\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_CLIENT_ID: \"$VITE_DISCORD_CLIENT_ID\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_REDIRECT_URI: \"$VITE_DISCORD_REDIRECT_URI\"," >> /usr/share/nginx/html/config.js
echo "  VITE_APP_TITLE: \"${VITE_APP_TITLE:-WAIFU.IM}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DOCS_URL: \"${VITE_DOCS_URL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_CONTACT_EMAIL: \"${VITE_CONTACT_EMAIL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_SERVER_URL: \"${VITE_DISCORD_SERVER_URL}\"" >> /usr/share/nginx/html/config.js
echo "};" >> /usr/share/nginx/html/config.js

# Execute the CMD
exec "$@"