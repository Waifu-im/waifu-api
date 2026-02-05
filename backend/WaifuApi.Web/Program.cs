using System.Reflection;
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
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;
using WaifuApi.Infrastructure;
using WaifuApi.Infrastructure.Persistence;
using WaifuApi.Web.Authentication;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Middleware;
using WaifuApi.Web.OpenApi;
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
        ConfigurationKeys.ApiBasePath,
        ConfigurationKeys.Frontend.BaseUrl,
        ConfigurationKeys.Cdn.BaseUrl,
        ConfigurationKeys.S3.AccessKey,
        ConfigurationKeys.S3.SecretKey,
        ConfigurationKeys.S3.ServiceUrl,
        ConfigurationKeys.S3.BucketName,
        ConfigurationKeys.S3.Region,
        ConfigurationKeys.Discord.ClientId,
        ConfigurationKeys.Discord.ClientSecret,
        ConfigurationKeys.Discord.RedirectUri,
        ConfigurationKeys.Jwt.Key,
        ConfigurationKeys.Jwt.Issuer,
        ConfigurationKeys.Jwt.Audience,
        ConfigurationKeys.Moderation.RequireImageReview,
        ConfigurationKeys.Moderation.RequireArtistReview,
        ConfigurationKeys.Moderation.RequireTagReview,
        ConfigurationKeys.Image.DefaultPageSize,
        ConfigurationKeys.Image.MaxPageSize,
        ConfigurationKeys.Image.MinWidth,
        ConfigurationKeys.Image.MinHeight,
        ConfigurationKeys.Image.MaxWidth,
        ConfigurationKeys.Image.MaxHeight,
        ConfigurationKeys.Image.AnimatedMinWidth,
        ConfigurationKeys.Image.AnimatedMinHeight,
        ConfigurationKeys.Image.AnimatedMaxWidth,
        ConfigurationKeys.Image.AnimatedMaxHeight,
        ConfigurationKeys.Tag.DefaultPageSize,
        ConfigurationKeys.Tag.MaxPageSize,
        ConfigurationKeys.Artist.DefaultPageSize,
        ConfigurationKeys.Artist.MaxPageSize,
        ConfigurationKeys.Album.DefaultPageSize,
        ConfigurationKeys.Album.MaxPageSize,
        ConfigurationKeys.User.DefaultPageSize,
        ConfigurationKeys.User.MaxPageSize,
        ConfigurationKeys.Review.DefaultPageSize,
        ConfigurationKeys.Review.MaxPageSize
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
builder.Services.AddControllers()
    .AddJsonOptions(options =>
    {
        options.JsonSerializerOptions.Converters.Add(new JsonStringEnumConverter());
    });
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

// Get version from assembly (set by MinVer)
var apiVersion = typeof(Program).Assembly
    .GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion ?? "0.0.0";

builder.Services.AddOpenApi(options =>
{
   options.AddSchemaTransformer<EnumSchemaTransformer>();
   options.AddSchemaTransformer<ErrorResponseSchemaTransformer>();
   options.AddSchemaTransformer<DescriptionSchemaTransformer>();
   options.AddOperationTransformer((operation, context, cancellationToken) =>
   {
       var transformer = new PaginationDescriptionTransformer(builder.Configuration);
       return transformer.TransformAsync(operation, context, cancellationToken);
   });
   options.AddOperationTransformer((operation, context, cancellationToken) =>
   {
       var transformer = new RoleRequirementTransformer(builder.Configuration);
       return transformer.TransformAsync(operation, context, cancellationToken);
   });
   options.AddDocumentTransformer<DescriptionSanitizerTransformer>();
   options.AddDocumentTransformer((document, context, cancellationToken) =>
    {
        document.Info = new OpenApiInfo { Title = "Waifu API", Version = apiVersion };

        // Only add server if basePath is not just "/" to avoid "//path" in docs
        if (!string.IsNullOrEmpty(basePath) && basePath != "/")
        {
            document.Servers = new List<OpenApiServer>
            {
                new OpenApiServer { Url = basePath }
            };
        }

        IOpenApiSecurityScheme bearerScheme = new OpenApiSecurityScheme
        {
            Type = SecuritySchemeType.Http,
            Scheme = "bearer",
            BearerFormat = "JWT",
            Description = "JWT token for authenticated users"
        };

        IOpenApiSecurityScheme apiKeyScheme = new OpenApiSecurityScheme
        {
            Type = SecuritySchemeType.ApiKey,
            In = ParameterLocation.Header,
            Name = "X-Api-Key",
            Description = "API Key for programmatic access"
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
builder.Services.AddScoped<IPermissionService, PermissionService>();

builder.Services.AddAuthentication(options =>
{
    options.DefaultAuthenticateScheme = "Smart";
    options.DefaultChallengeScheme = "Smart";
})
.AddPolicyScheme("Smart", "Bearer or ApiKey", options =>
{
    options.ForwardDefaultSelector = context =>
    {
        if (context.Request.Headers.ContainsKey("X-Api-Key"))
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
    options.AddPolicy(AuthorizationPolicies.Admin, policy => policy.RequireRole(Role.Admin.ToString()));
    options.AddPolicy(AuthorizationPolicies.Moderator, policy => policy.RequireAssertion(context =>
        context.User.HasClaim(c => c.Type == System.Security.Claims.ClaimTypes.Role &&
        (c.Value == Role.Admin.ToString() || c.Value == Role.Moderator.ToString()))));
});

// Register the background service for stats cleanup
builder.Services.AddHostedService<StatsCleanupService>();

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

app.MapOpenApi();
app.MapScalarApiReference();

app.UseCors("AllowAll");


// IMPORTANT: Authentication must come BEFORE RequestLoggingMiddleware
// otherwise HttpContext.User will not be populated when logging runs.
app.UseAuthentication();
app.UseAuthorization();

app.UseMiddleware<RequestLoggingMiddleware>();
app.UseMiddleware<ExceptionHandlingMiddleware>();

app.UseHttpsRedirection();
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
