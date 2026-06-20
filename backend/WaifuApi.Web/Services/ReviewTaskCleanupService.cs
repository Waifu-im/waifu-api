using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Web.Services;

/// <summary>
/// Periodically prunes resolved (approved/rejected/cancelled) review tasks older than the configured
/// retention window. Pending tasks are never touched. Content attribution (UploaderId/CreatorId) lives on the
/// entities, not on the task, so pruning never affects it.
/// </summary>
public class ReviewTaskCleanupService : BackgroundService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<ReviewTaskCleanupService> _logger;
    private readonly int _retentionDays;

    public ReviewTaskCleanupService(IServiceProvider serviceProvider, ILogger<ReviewTaskCleanupService> logger, IConfiguration configuration)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
        _retentionDays = int.TryParse(configuration[ConfigurationKeys.Review.TaskRetentionDays], out var days) ? days : 0;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        if (_retentionDays <= 0)
        {
            _logger.LogInformation("Review task pruning is disabled (Review:TaskRetentionDays <= 0).");
            return;
        }

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                using var scope = _serviceProvider.CreateScope();
                var dbContext = scope.ServiceProvider.GetRequiredService<IWaifuDbContext>();

                var cutoff = DateTime.UtcNow.AddDays(-_retentionDays);

                var deletedCount = await dbContext.ReviewTasks
                    .Where(t => t.Status != ReviewTaskStatus.Pending && t.ResolvedAt != null && t.ResolvedAt < cutoff)
                    .ExecuteDeleteAsync(stoppingToken);

                if (deletedCount > 0)
                {
                    _logger.LogInformation("Pruned {Count} resolved review tasks older than {Days} days", deletedCount, _retentionDays);
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Error occurred while pruning review tasks");
            }

            await Task.Delay(TimeSpan.FromHours(24), stoppingToken);
        }
    }
}
