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

            // 1. Atomic upsert DailyStat
            await AtomicUpsertDailyStat(dbContext, today);

            // 2. Atomic upsert GlobalStat
            await AtomicUpsertGlobalStat(dbContext);

            // 3. Atomic increment User Stats if authenticated
            if (userId.HasValue)
            {
                await dbContext.Users
                    .Where(u => u.Id == userId.Value)
                    .ExecuteUpdateAsync(s => s
                        .SetProperty(u => u.RequestCount, u => u.RequestCount + 1)
                        .SetProperty(u => u.ApiKeyRequestCount, u => u.ApiKeyRequestCount + (isApiKey ? 1 : 0))
                        .SetProperty(u => u.JwtRequestCount, u => u.JwtRequestCount + (isJwt ? 1 : 0)));
            }
        }
        catch
        {
            // Silently fail logging to not affect application stability
        }
    }

    private static async Task AtomicUpsertDailyStat(IWaifuDbContext dbContext, DateOnly today)
    {
        var updated = await dbContext.DailyStats
            .Where(s => s.Date == today)
            .ExecuteUpdateAsync(s => s.SetProperty(p => p.RequestCount, p => p.RequestCount + 1));

        if (updated == 0)
        {
            try
            {
                dbContext.DailyStats.Add(new DailyStat { Date = today, RequestCount = 1 });
                await dbContext.SaveChangesAsync(CancellationToken.None);
            }
            catch (DbUpdateException)
            {
                // Another request already inserted for today, just increment
                await dbContext.DailyStats
                    .Where(s => s.Date == today)
                    .ExecuteUpdateAsync(s => s.SetProperty(p => p.RequestCount, p => p.RequestCount + 1));
            }
        }
    }

    private static async Task AtomicUpsertGlobalStat(IWaifuDbContext dbContext)
    {
        var updated = await dbContext.GlobalStats
            .Where(s => s.Key == "TotalRequests")
            .ExecuteUpdateAsync(s => s.SetProperty(p => p.Value, p => p.Value + 1));

        if (updated == 0)
        {
            try
            {
                dbContext.GlobalStats.Add(new GlobalStat { Key = "TotalRequests", Value = 1 });
                await dbContext.SaveChangesAsync(CancellationToken.None);
            }
            catch (DbUpdateException)
            {
                // Another request already inserted the key, just increment
                await dbContext.GlobalStats
                    .Where(s => s.Key == "TotalRequests")
                    .ExecuteUpdateAsync(s => s.SetProperty(p => p.Value, p => p.Value + 1));
            }
        }
    }
}