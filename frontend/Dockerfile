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

# Copy Nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the script to generate config.js at runtime
COPY ./public/config.js /usr/share/nginx/html/config.js.template

# Copy the entrypoint script from the host
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Fix line endings (CRLF -> LF) for files that might have Windows line endings
RUN sed -i 's/\r$//' /docker-entrypoint.sh && \
    sed -i 's/\r$//' /etc/nginx/conf.d/default.conf && \
    chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
