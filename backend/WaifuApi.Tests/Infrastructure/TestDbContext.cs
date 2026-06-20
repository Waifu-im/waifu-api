using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Storage;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Tests.Infrastructure;

/// <summary>
/// A lightweight EF Core InMemory context implementing <see cref="IWaifuDbContext"/> for unit tests.
/// Deliberately free of Postgres-specific mapping (jsonb, bit(64), partial indexes) so the in-memory
/// provider can host it; those are database concerns validated separately.
/// </summary>
public class TestDbContext : DbContext, IWaifuDbContext
{
    public TestDbContext(DbContextOptions<TestDbContext> options) : base(options) { }

    public DbSet<User> Users => Set<User>();
    public DbSet<Image> Images => Set<Image>();
    public DbSet<Artist> Artists => Set<Artist>();
    public DbSet<Tag> Tags => Set<Tag>();
    public DbSet<ApiKey> ApiKeys => Set<ApiKey>();
    public DbSet<Album> Albums => Set<Album>();
    public DbSet<AlbumItem> AlbumItems => Set<AlbumItem>();
    public DbSet<Report> Reports => Set<Report>();
    public DbSet<ReviewTask> ReviewTasks => Set<ReviewTask>();
    public DbSet<DailyStat> DailyStats => Set<DailyStat>();
    public DbSet<GlobalStat> GlobalStats => Set<GlobalStat>();

    public Task<IDbContextTransaction> BeginTransactionAsync(CancellationToken cancellationToken)
        => Database.BeginTransactionAsync(cancellationToken);

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.Entity<AlbumItem>().HasKey(e => new { e.AlbumId, e.ImageId });
        modelBuilder.Entity<GlobalStat>().HasKey(e => e.Key);

        // BitArray has no InMemory scalar mapping (it is a Postgres bit(64) column in production), so it is
        // round-tripped through the project's own hex helper for tests.
        modelBuilder.Entity<Image>()
            .Property(e => e.PerceptualHash)
            .HasConversion(v => BitArrayHelper.ToHex(v), v => BitArrayHelper.FromHex(v));

        modelBuilder.Entity<Image>().HasMany(e => e.Artists).WithMany(a => a.Images);
        modelBuilder.Entity<Image>().HasMany(i => i.Tags).WithMany(t => t.Images);
        modelBuilder.Entity<Image>().HasOne(e => e.Uploader).WithMany().HasForeignKey(e => e.UploaderId);
        modelBuilder.Entity<Tag>().HasOne(e => e.Creator).WithMany().HasForeignKey(e => e.CreatorId);
        modelBuilder.Entity<Artist>().HasOne(e => e.Creator).WithMany().HasForeignKey(e => e.CreatorId);

        modelBuilder.Entity<ReviewTask>().HasOne(e => e.Submitter).WithMany().HasForeignKey(e => e.SubmitterId);
        modelBuilder.Entity<ReviewTask>().HasOne(e => e.Reviewer).WithMany().HasForeignKey(e => e.ReviewerId);
    }
}
