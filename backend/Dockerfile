# Build stage
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src

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

# Build and publish
WORKDIR "/src/WaifuApi.Web"
RUN dotnet publish "WaifuApi.Web.csproj" -c Release -o /app/publish /p:UseAppHost=false

# Runtime stage
FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS final
WORKDIR /app
EXPOSE 8080
EXPOSE 8443

COPY --from=build /app/publish .
ENTRYPOINT ["dotnet", "WaifuApi.Web.dll"]
