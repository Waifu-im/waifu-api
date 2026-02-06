# Stage 1: Generate OpenAPI spec from backend
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS openapi
WORKDIR /src

ARG VERSION=0.0.0
ARG API_BASE_URL

# Copy backend source
COPY backend/WaifuApi.slnx ./
COPY backend/WaifuApi.Web/WaifuApi.Web.csproj WaifuApi.Web/
COPY backend/WaifuApi.Application/WaifuApi.Application.csproj WaifuApi.Application/
COPY backend/WaifuApi.Domain/WaifuApi.Domain.csproj WaifuApi.Domain/
COPY backend/WaifuApi.Infrastructure/WaifuApi.Infrastructure.csproj WaifuApi.Infrastructure/

RUN dotnet restore "WaifuApi.Web/WaifuApi.Web.csproj"

COPY backend/ .

# Generate OpenAPI spec with CI environment
ENV ASPNETCORE_ENVIRONMENT=CI
ENV API_BASE_URL=${API_BASE_URL}
RUN dotnet build WaifuApi.Web/WaifuApi.Web.csproj -c Release /p:OpenApiGenerateDocumentsOnBuild=true /p:Version=${VERSION}

# Stage 2: Build documentation
FROM node:20-alpine AS build
WORKDIR /app

ARG DOCS_URL
ARG DOCS_BASE_URL
ARG DOCS_ORG_NAME
ARG DOCS_PROJECT_NAME
ARG API_BASE_URL
ARG APP_NAME
ARG APP_TAGLINE
ARG FRONTEND_URL
ARG GITHUB_URL

ENV DOCS_URL=${DOCS_URL}
ENV DOCS_BASE_URL=${DOCS_BASE_URL}
ENV DOCS_ORG_NAME=${DOCS_ORG_NAME}
ENV DOCS_PROJECT_NAME=${DOCS_PROJECT_NAME}
ENV API_BASE_URL=${API_BASE_URL}
ENV APP_NAME=${APP_NAME}
ENV APP_TAGLINE=${APP_TAGLINE}
ENV FRONTEND_URL=${FRONTEND_URL}
ENV GITHUB_URL=${GITHUB_URL}

# Copy package files
COPY docs/package*.json ./

RUN npm ci

# Copy docs source
COPY docs/ .

# Copy generated OpenAPI spec from previous stage
COPY --from=openapi /openapi/WaifuApi.Web.json ../openapi/

# Generate API docs from OpenAPI spec
RUN npx docusaurus gen-api-docs all

# Build static site
RUN npm run build

# Stage 3: Serve with nginx
FROM nginx:alpine AS final

COPY --from=build /app/build /usr/share/nginx/html

# Custom nginx config for SPA routing
RUN echo 'server { \
    listen 80; \
    root /usr/share/nginx/html; \
    index index.html; \
    location / { \
        try_files $uri $uri/ /index.html; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
