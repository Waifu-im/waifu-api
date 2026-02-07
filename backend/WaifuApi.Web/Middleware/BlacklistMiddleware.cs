using System.Security.Claims;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Web.Middleware;

public class BlacklistMiddleware
{
    private readonly RequestDelegate _next;
    private readonly IServiceScopeFactory _scopeFactory;

    public BlacklistMiddleware(RequestDelegate next, IServiceScopeFactory scopeFactory)
    {
        _next = next;
        _scopeFactory = scopeFactory;
    }

    public async Task InvokeAsync(HttpContext context)
    {
        if (!long.TryParse(context.User.FindFirst(ClaimTypes.NameIdentifier)?.Value, out var userId))
        {
            await _next(context);
            return;
        }

        using var scope = _scopeFactory.CreateScope();
        var dbContext = scope.ServiceProvider.GetRequiredService<IWaifuDbContext>();

        var blacklistInfo = await dbContext.Users
            .Where(u => u.Id == userId)
            .Select(u => new { u.IsBlacklisted, u.BlacklistReason })
            .FirstOrDefaultAsync();

        if (blacklistInfo is { IsBlacklisted: true })
        {
            var detail = string.IsNullOrEmpty(blacklistInfo.BlacklistReason)
                ? "Your account has been blacklisted."
                : $"Your account has been blacklisted. Reason: {blacklistInfo.BlacklistReason}";

            throw new ForbiddenException(detail);
        }

        await _next(context);
    }
}
