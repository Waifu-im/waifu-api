# Build stage
FROM node:20-alpine AS build
WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Runtime stage
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the script to generate config.js at runtime
COPY ./public/config.js /usr/share/nginx/html/config.js.template

# Create an entrypoint script to substitute env vars
RUN echo '#!/bin/sh' > /docker-entrypoint.sh && \
    echo 'echo "window.env = {" > /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_API_URL: \"$VITE_API_URL\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_DISCORD_CLIENT_ID: \"$VITE_DISCORD_CLIENT_ID\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_DISCORD_REDIRECT_URI: \"$VITE_DISCORD_REDIRECT_URI\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_APP_TITLE: \"${VITE_APP_TITLE:-WAIFU.IM}\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_DOCS_URL: \"${VITE_DOCS_URL}\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_CONTACT_EMAIL: \"${VITE_CONTACT_EMAIL}\"," >> /usr/share/nginx/html/config.js' && \
    echo 'echo "  VITE_DISCORD_SERVER_URL: \"${VITE_DISCORD_SERVER_URL}\"" >> /usr/share/nginx/html/config.js' && \
    echo 'echo "};" >> /usr/share/nginx/html/config.js' && \
    echo 'exec "$@"' >> /docker-entrypoint.sh && \
    chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
