using System.Security.Claims;
using System.Text.Encodings.Web;
using Mediator;
using Microsoft.AspNetCore.Authentication;
using Microsoft.Extensions.Options;
using WaifuApi.Application.Features.Auth.ValidateApiKey; // Assurez-vous d'avoir créé cette Query

namespace WaifuApi.Web.Authentication;

public class ApiKeyAuthenticationOptions : AuthenticationSchemeOptions
{
}

public class ApiKeyAuthenticationHandler : AuthenticationHandler<ApiKeyAuthenticationOptions>
{
    private readonly IMediator _mediator;
    private readonly TimeProvider _timeProvider;

    public ApiKeyAuthenticationHandler(
        IOptionsMonitor<ApiKeyAuthenticationOptions> options,
        ILoggerFactory logger,
        UrlEncoder encoder,
        IMediator mediator,
        TimeProvider timeProvider)
        : base(options, logger, encoder)
    {
        _mediator = mediator;
        _timeProvider = timeProvider;
    }

    protected override async Task<AuthenticateResult> HandleAuthenticateAsync()
    {
        if (!Request.Headers.TryGetValue("Authorization", out var authHeader))
        {
            return AuthenticateResult.NoResult();
        }

        var headerValue = authHeader.FirstOrDefault();
        if (string.IsNullOrEmpty(headerValue) || !headerValue.StartsWith("ApiKey "))
        {
            return AuthenticateResult.NoResult();
        }

        var providedApiKey = headerValue.Substring("ApiKey ".Length).Trim();

        if (string.IsNullOrEmpty(providedApiKey))
        {
            return AuthenticateResult.Fail("Empty API Key");
        }
        
        var apiKeyEntity = await _mediator.Send(new ValidateApiKeyQuery(providedApiKey));

        if (apiKeyEntity == null)
        {
            return AuthenticateResult.Fail("Invalid API Key");
        }

        if (apiKeyEntity.ExpirationDate.HasValue && 
            apiKeyEntity.ExpirationDate.Value < _timeProvider.GetUtcNow().UtcDateTime)
        {
            return AuthenticateResult.Fail("API Key Expired");
        }

        var claims = new[]
        {
            new Claim(ClaimTypes.NameIdentifier, apiKeyEntity.UserId.ToString()),
            new Claim(ClaimTypes.Name, apiKeyEntity.User.Name), 
            new Claim("ApiKeyId", apiKeyEntity.Id.ToString()),
            new Claim(ClaimTypes.Role, apiKeyEntity.User.Role.ToString()) 
        };

        var identity = new ClaimsIdentity(claims, Scheme.Name);
        var principal = new ClaimsPrincipal(identity);
        var ticket = new AuthenticationTicket(principal, Scheme.Name);

        return AuthenticateResult.Success(ticket);
    }

    protected override async Task HandleChallengeAsync(AuthenticationProperties properties)
    {
        Response.StatusCode = 401;
        Response.ContentType = "application/problem+json";
        await Response.WriteAsJsonAsync(new 
        {
            Title = "Unauthorized",
            Status = 401,
            Detail = "You are not authorized to access this resource. Please provide a valid API Key or Bearer Token.",
            Type = "https://tools.ietf.org/html/rfc9110#section-15.5.2"
        });
    }

    protected override async Task HandleForbiddenAsync(AuthenticationProperties properties)
    {
        Response.StatusCode = 403;
        Response.ContentType = "application/problem+json";
        await Response.WriteAsJsonAsync(new 
        {
            Title = "Forbidden",
            Status = 403,
            Detail = "You do not have permission to access this resource.",
            Type = "https://tools.ietf.org/html/rfc9110#section-15.5.4"
        });
    }
}