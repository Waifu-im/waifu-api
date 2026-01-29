using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using Microsoft.IdentityModel.Tokens;
using Microsoft.OpenApi;
using Scalar.AspNetCore;
using WaifuApi.Application;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Infrastructure;
using WaifuApi.Infrastructure.Persistence;
using WaifuApi.Web.Authentication;
using WaifuApi.Web.Middleware;
using WaifuApi.Web.Services;

Console.WriteLine($"Current Directory: {Directory.GetCurrentDirectory()}");

var currentDir = Directory.GetCurrentDirectory();
string? envPath = null;
int levels = 0;

while (currentDir != null && levels <= 2)
{
    var path = Path.Combine(currentDir, ".env");
    if (File.Exists(path))
    {
        envPath = path;
        break;
    }
    currentDir = Directory.GetParent(currentDir)?.FullName;
    levels++;
}

if (envPath != null)
{
    Console.WriteLine($"Loading .env from: {envPath}");
    DotNetEnv.Env.Load(envPath);
}
else
{
    Console.WriteLine("No .env file found in current directory or 2 levels up.");
}

var builder = WebApplication.CreateBuilder(args);

void ValidateConfiguration(IConfiguration configuration)
{
    var requiredKeys = new[]
    {
        "API_BASE_PATH",
        "Frontend:BaseUrl",
        "Cdn:BaseUrl",
        "S3:AccessKey",
        "S3:SecretKey",
        "S3:ServiceUrl",
        "S3:BucketName",
        "S3:Region",
        "Discord:ClientId",
        "Discord:ClientSecret",
        "Discord:RedirectUri",
        "Jwt:Key",
        "Jwt:Issuer",
        "Jwt:Audience",
        "Moderation:RequireImageReview",
        "Moderation:RequireArtistReview",
        "Moderation:RequireTagReview",
        "Image:DefaultPageSize",
        "Image:MaxPageSize",
        "Image:MinWidth",
        "Image:MinHeight",
        "Image:MaxWidth",
        "Image:MaxHeight",
        "Tag:DefaultPageSize",
        "Tag:MaxPageSize",
        "Artist:DefaultPageSize",
        "Artist:MaxPageSize",
        "Album:DefaultPageSize",
        "Album:MaxPageSize",
        "User:DefaultPageSize",
        "User:MaxPageSize",
        "Review:DefaultPageSize",
        "Review:MaxPageSize"
    };

    var missingKeys = new List<string>();
    foreach (var key in requiredKeys)
    {
        if (string.IsNullOrEmpty(configuration[key]))
        {
            missingKeys.Add(key);
        }
    }

    if (missingKeys.Any())
    {
        throw new InvalidOperationException($"Missing required configuration keys: {string.Join(", ", missingKeys)}");
    }
}

ValidateConfiguration(builder.Configuration);

var connectionString = builder.Configuration.GetConnectionString("DefaultConnection");
Console.WriteLine($"Using Connection String: {connectionString}");

var basePath = builder.Configuration["API_BASE_PATH"];

builder.Services.AddApplication();
builder.Services.AddInfrastructure(builder.Configuration);
builder.Services.AddControllers();
builder.Services.AddSingleton(TimeProvider.System);

builder.Services.AddCors(options =>
{
    options.AddPolicy("AllowAll",
        builder =>
        {
            builder.AllowAnyOrigin()
                   .AllowAnyMethod()
                   .AllowAnyHeader();
        });
});

builder.Services.AddOpenApi(options =>
{
   options.AddDocumentTransformer((document, context, cancellationToken) =>
    {
        document.Info = new OpenApiInfo { Title = "Waifu API", Version = "v1" };
        
        document.Servers = new List<OpenApiServer>
        {
            new OpenApiServer { Url = basePath }
        };

        IOpenApiSecurityScheme bearerScheme = new OpenApiSecurityScheme
        {
            Type = SecuritySchemeType.Http,
            Scheme = "bearer",
            BearerFormat = "JWT",
            In = ParameterLocation.Header,
            Description = "Entrez votre token JWT ici"
        };

        IOpenApiSecurityScheme apiKeyScheme = new OpenApiSecurityScheme
        {
            Type = SecuritySchemeType.ApiKey,
            Name = "Authorization",
            In = ParameterLocation.Header,
            Description = "Entrez votre ApiKey (Format: ApiKey {votre_clé})"
        };

        document.Components ??= new OpenApiComponents();
        
        document.Components.SecuritySchemes = new Dictionary<string, IOpenApiSecurityScheme>
        {
            ["Bearer"] = bearerScheme,
            ["ApiKey"] = apiKeyScheme
        };
        
        var requirement = new OpenApiSecurityRequirement
        {
            [new OpenApiSecuritySchemeReference("Bearer", document)] = new List<string>(),
            [new OpenApiSecuritySchemeReference("ApiKey", document)] = new List<string>()
        };

        document.Security = new List<OpenApiSecurityRequirement> { requirement };

        return Task.CompletedTask;
    });
});


builder.Services.AddHttpContextAccessor();
builder.Services.AddScoped<ICurrentUserService, CurrentUserService>();

builder.Services.AddAuthentication(options =>
{
    options.DefaultAuthenticateScheme = "Smart";
    options.DefaultChallengeScheme = "Smart";
})
.AddPolicyScheme("Smart", "Bearer or ApiKey", options =>
{
    options.ForwardDefaultSelector = context =>
    {
        var authHeader = context.Request.Headers["Authorization"].FirstOrDefault();
        if (authHeader?.StartsWith("ApiKey ") == true)
        {
            return "ApiKey";
        }
        return JwtBearerDefaults.AuthenticationScheme;
    };
})
.AddJwtBearer(options =>
{
    options.TokenValidationParameters = new TokenValidationParameters
    {
        ValidateIssuer = true,
        ValidateAudience = true,
        ValidateLifetime = true,
        ValidateIssuerSigningKey = true,
        ValidIssuer = builder.Configuration["Jwt:Issuer"],
        ValidAudience = builder.Configuration["Jwt:Audience"],
        IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes(builder.Configuration["Jwt:Key"]!))
    };
})
.AddScheme<ApiKeyAuthenticationOptions, ApiKeyAuthenticationHandler>("ApiKey", null);

builder.Services.AddAuthorization(options =>
{
    options.AddPolicy("Admin", policy => policy.RequireRole(Role.Admin.ToString()));
    options.AddPolicy("Moderator", policy => policy.RequireAssertion(context => 
        context.User.HasClaim(c => c.Type == System.Security.Claims.ClaimTypes.Role && 
        (c.Value == Role.Admin.ToString() || c.Value == Role.Moderator.ToString()))));
});

var app = builder.Build();

app.UsePathBase(basePath);

using (var scope = app.Services.CreateScope())
{
    try 
    {
        var dbContext = scope.ServiceProvider.GetRequiredService<WaifuDbContext>();
        dbContext.Database.Migrate();
    }
    catch (Exception ex)
    {
        Console.WriteLine($"Migration failed: {ex.Message}");
    }
}

if (app.Environment.IsDevelopment())
{
    app.MapOpenApi();
    app.MapScalarApiReference();
}

app.UseMiddleware<ExceptionHandlingMiddleware>();

app.UseCors("AllowAll");

app.UseHttpsRedirection();
app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();

app.MapFallback(async context =>
{
    context.Response.StatusCode = StatusCodes.Status404NotFound;
    context.Response.ContentType = "application/problem+json";

    var problemDetails = new ProblemDetails
    {
        Title = "Not Found",
        Status = StatusCodes.Status404NotFound,
        Detail = "The requested endpoint was not found.",
        Instance = context.Request.Path,
        Type = "https://tools.ietf.org/html/rfc9110#section-15.5.5"
    };
    
    problemDetails.Extensions["traceId"] = System.Diagnostics.Activity.Current?.Id ?? context.TraceIdentifier;

    await context.Response.WriteAsync(JsonSerializer.Serialize(problemDetails));
});
app.Run();
