using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Web.Services;

public class StatsCleanupService : BackgroundService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<StatsCleanupService> _logger;

    public StatsCleanupService(IServiceProvider serviceProvider, ILogger<StatsCleanupService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                using var scope = _serviceProvider.CreateScope();
                var dbContext = scope.ServiceProvider.GetRequiredService<IWaifuDbContext>();

                var cutoffDate = DateOnly.FromDateTime(DateTime.UtcNow.AddYears(-1));
                
                // Delete stats older than 1 year
                if (dbContext is DbContext efContext)
                {
                    var deletedCount = await efContext.Database.ExecuteSqlRawAsync(
                        "DELETE FROM \"DailyStats\" WHERE \"Date\" < {0}", 
                        new object[] { cutoffDate }, 
                        stoppingToken
                    );
                    
                    if (deletedCount > 0)
                    {
                        _logger.LogInformation("Cleaned up {Count} old daily stats records", deletedCount);
                    }
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error occurred while cleaning up stats");
            }

            // Run once every 24 hours
            await Task.Delay(TimeSpan.FromHours(24), stoppingToken);
        }
    }
}