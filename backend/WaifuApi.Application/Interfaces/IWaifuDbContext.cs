using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Storage;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Interfaces;

public interface IWaifuDbContext
{
    DbSet<User> Users { get; }
    DbSet<Image> Images { get; }
    DbSet<Artist> Artists { get; }
    DbSet<Tag> Tags { get; }
    DbSet<ApiKey> ApiKeys { get; }
    DbSet<Album> Albums { get; }
    DbSet<AlbumItem> AlbumItems { get; }
    DbSet<Report> Reports { get; }
    DbSet<ReviewTask> ReviewTasks { get; }
    DbSet<DailyStat> DailyStats { get; }
    DbSet<GlobalStat> GlobalStats { get; }

    Task<int> SaveChangesAsync(CancellationToken cancellationToken);

    /// <summary>Begins a database transaction so multi-step operations (e.g. apply + resolve) commit atomically.</summary>
    Task<IDbContextTransaction> BeginTransactionAsync(CancellationToken cancellationToken);
}