# Build stage
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src

# MinVer version override (calculated from git tags in CI)
ARG MINVER_VERSION=0.0.0-local

# Copy solution and project files
COPY ["WaifuApi.slnx", "./"]
COPY ["WaifuApi.Web/WaifuApi.Web.csproj", "WaifuApi.Web/"]
COPY ["WaifuApi.Application/WaifuApi.Application.csproj", "WaifuApi.Application/"]
COPY ["WaifuApi.Domain/WaifuApi.Domain.csproj", "WaifuApi.Domain/"]
COPY ["WaifuApi.Infrastructure/WaifuApi.Infrastructure.csproj", "WaifuApi.Infrastructure/"]

# Restore dependencies
RUN dotnet restore "WaifuApi.Web/WaifuApi.Web.csproj"

# Copy the rest of the source code
COPY . .

# Build and publish with MinVer version override
WORKDIR "/src/WaifuApi.Web"
RUN dotnet publish "WaifuApi.Web.csproj" -c Release -o /app/publish /p:UseAppHost=false /p:MinVerVersionOverride=${MINVER_VERSION} /p:OpenApiGenerateDocumentsOnBuild=false

# Runtime stage
FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS final
WORKDIR /app

# Install Kerberos library to suppress Npgsql GSSAPI warning
RUN apt-get update && apt-get install -y --no-install-recommends libgssapi-krb5-2 && rm -rf /var/lib/apt/lists/*

COPY --from=build /app/publish .
ENTRYPOINT ["dotnet", "WaifuApi.Web.dll"]
