using System.Security.Claims;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Middleware;

public class RequestLoggingMiddleware
{
    private readonly RequestDelegate _next;
    private readonly IServiceScopeFactory _scopeFactory;

    public RequestLoggingMiddleware(RequestDelegate next, IServiceScopeFactory scopeFactory)
    {
        _next = next;
        _scopeFactory = scopeFactory;
    }

    public async Task InvokeAsync(HttpContext context)
    {
        // Capture user ID before response is sent (context might be disposed/modified)
        long? userId = null;
        if (long.TryParse(context.User.FindFirst(ClaimTypes.NameIdentifier)?.Value, out var id))
        {
            userId = id;
        }

        // Determine auth type
        bool isApiKey = context.User.HasClaim(c => c.Type == "ApiKeyId");
        bool isJwt = !isApiKey && userId.HasValue; // If authenticated but not via ApiKey, assume JWT (Discord)

        // 1. Process the request first
        await _next(context);

        // 2. Fire and forget logging AFTER response is sent
        _ = Task.Run(() => LogRequestAsync(userId, isApiKey, isJwt));
    }

    private async Task LogRequestAsync(long? userId, bool isApiKey, bool isJwt)
    {
        try
        {
            using var scope = _scopeFactory.CreateScope();
            var dbContext = scope.ServiceProvider.GetRequiredService<IWaifuDbContext>();

            var today = DateOnly.FromDateTime(DateTime.UtcNow);
            
            // 1. Upsert DailyStat
            var dailyStat = await dbContext.DailyStats.FirstOrDefaultAsync(s => s.Date == today);
            if (dailyStat == null)
            {
                dailyStat = new DailyStat { Date = today, RequestCount = 1 };
                dbContext.DailyStats.Add(dailyStat);
            }
            else
            {
                dailyStat.RequestCount++;
            }
            
            // 2. Upsert GlobalStat
            var globalStat = await dbContext.GlobalStats.FirstOrDefaultAsync(s => s.Key == "TotalRequests");
            if (globalStat == null)
            {
                globalStat = new GlobalStat { Key = "TotalRequests", Value = 1 };
                dbContext.GlobalStats.Add(globalStat);
            }
            else
            {
                globalStat.Value++;
            }

            // 3. Increment User Stats if authenticated
            if (userId.HasValue)
            {
                var user = await dbContext.Users.FindAsync(userId.Value);
                if (user != null)
                {
                    user.RequestCount++;
                    if (isApiKey) user.ApiKeyRequestCount++;
                    if (isJwt) user.JwtRequestCount++;
                }
            }

            await dbContext.SaveChangesAsync(CancellationToken.None);
        }
        catch
        {
            // Silently fail logging to not affect application stability
        }
    }
}