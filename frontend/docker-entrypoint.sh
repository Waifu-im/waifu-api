#!/bin/sh

# Recreate config.js with environment variables
echo "window.env = {" > /usr/share/nginx/html/config.js
echo "  VITE_API_URL: \"$VITE_API_URL\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_CLIENT_ID: \"$VITE_DISCORD_CLIENT_ID\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_REDIRECT_URI: \"$VITE_DISCORD_REDIRECT_URI\"," >> /usr/share/nginx/html/config.js
echo "  VITE_APP_TITLE: \"${VITE_APP_TITLE:-WAIFU.IM}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DOCS_URL: \"${VITE_DOCS_URL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_CONTACT_EMAIL: \"${VITE_CONTACT_EMAIL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_DISCORD_SERVER_URL: \"${VITE_DISCORD_SERVER_URL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_STATUS_URL: \"${VITE_STATUS_URL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_GITHUB_URL: \"${VITE_GITHUB_URL}\"," >> /usr/share/nginx/html/config.js
echo "  VITE_TOS_URL: \"${VITE_TOS_URL}\"" >> /usr/share/nginx/html/config.js
echo "};" >> /usr/share/nginx/html/config.js

# Generate site.webmanifest with environment variables
APP_TITLE="${VITE_APP_TITLE:-Waifu.im}"
cat > /usr/share/nginx/html/site.webmanifest <<EOF
{
  "name": "$APP_TITLE",
  "short_name": "$APP_TITLE",
  "description": "The API for your Waifu content",
  "icons": [
    {
      "src": "/android-chrome-192x192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/android-chrome-512x512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ],
  "theme_color": "#ffffff",
  "background_color": "#ffffff",
  "display": "standalone",
  "start_url": "/"
}
EOF

# Execute the CMD
exec "$@"