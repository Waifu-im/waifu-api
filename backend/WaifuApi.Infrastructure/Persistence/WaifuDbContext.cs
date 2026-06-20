using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.ChangeTracking;
using Microsoft.EntityFrameworkCore.Storage;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Infrastructure.Persistence;

public class WaifuDbContext : DbContext, IWaifuDbContext
{
    public WaifuDbContext(DbContextOptions<WaifuDbContext> options) : base(options)
    {
    }

    public DbSet<User> Users { get; set; }
    public DbSet<Image> Images { get; set; }
    public DbSet<Artist> Artists { get; set; }
    public DbSet<Tag> Tags { get; set; }
    public DbSet<ApiKey> ApiKeys { get; set; }
    public DbSet<Album> Albums { get; set; }
    public DbSet<AlbumItem> AlbumItems { get; set; }
    public DbSet<Report> Reports { get; set; }
    public DbSet<ReviewTask> ReviewTasks { get; set; }
    public DbSet<DailyStat> DailyStats { get; set; }
    public DbSet<GlobalStat> GlobalStats { get; set; }

    public Task<IDbContextTransaction> BeginTransactionAsync(CancellationToken cancellationToken)
        => Database.BeginTransactionAsync(cancellationToken);

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.HasPostgresExtension("vector");

        modelBuilder.Entity<User>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Name).IsRequired();
            entity.Property(e => e.DiscordId).IsRequired();
            entity.HasIndex(e => e.DiscordId).IsUnique();
        });

        modelBuilder.Entity<ApiKey>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.KeyHash).IsRequired();
            entity.HasIndex(e => e.KeyHash).IsUnique();
            entity.HasOne(e => e.User)
                .WithMany(u => u.ApiKeys)
                .HasForeignKey(e => e.UserId)
                .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<Image>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.PerceptualHash)
                .HasColumnType("bit(64)")
                .IsRequired();
            
            entity.Property(e => e.Extension).IsRequired();
            entity.Property(e => e.DominantColor).IsRequired();
            
            entity.HasOne(e => e.Uploader)
                .WithMany()
                .HasForeignKey(e => e.UploaderId)
                .OnDelete(DeleteBehavior.SetNull);

            entity.HasMany(e => e.Artists)
                .WithMany(a => a.Images);

            entity.HasMany(i => i.Tags)
                .WithMany(t => t.Images);
        });

        modelBuilder.Entity<Artist>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Name).IsRequired();
            entity.HasIndex(e => e.Name).IsUnique();
            entity.HasIndex(e => e.Patreon).IsUnique();
            entity.HasIndex(e => e.Pixiv).IsUnique();
            entity.HasIndex(e => e.Twitter).IsUnique();
            entity.HasIndex(e => e.DeviantArt).IsUnique();
            entity.HasOne(e => e.Creator)
                .WithMany()
                .HasForeignKey(e => e.CreatorId)
                .OnDelete(DeleteBehavior.SetNull);
        });

        modelBuilder.Entity<Tag>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Name).IsRequired();
            entity.HasIndex(e => e.Name).IsUnique();

            entity.Property(e => e.Slug).IsRequired();
            entity.HasIndex(e => e.Slug).IsUnique();

            entity.Property(e => e.Description).IsRequired();
            entity.HasOne(e => e.Creator)
                .WithMany()
                .HasForeignKey(e => e.CreatorId)
                .OnDelete(DeleteBehavior.SetNull);
        });

        modelBuilder.Entity<Album>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Name).IsRequired();
            entity.HasOne(e => e.User)
                .WithMany()
                .HasForeignKey(e => e.UserId)
                .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<AlbumItem>(entity =>
        {
            entity.HasKey(e => new { e.AlbumId, e.ImageId });
            entity.HasOne(e => e.Album)
                .WithMany(a => a.Items)
                .HasForeignKey(e => e.AlbumId)
                .OnDelete(DeleteBehavior.Cascade);
            entity.HasOne(e => e.Image)
                .WithMany()
                .HasForeignKey(e => e.ImageId)
                .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<Report>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Description).IsRequired();
            entity.HasOne(e => e.User)
                .WithMany()
                .HasForeignKey(e => e.UserId)
                .OnDelete(DeleteBehavior.Cascade);
            entity.HasOne(e => e.Image)
                .WithMany()
                .HasForeignKey(e => e.ImageId)
                .OnDelete(DeleteBehavior.SetNull);
        });

        modelBuilder.Entity<ReviewTask>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Payload).HasColumnType("jsonb");      // nullable: NewContent tasks have no payload

            // Submitter/Reviewer are SET NULL so the durable audit record survives user deletion.
            entity.HasOne(e => e.Submitter)
                .WithMany()
                .HasForeignKey(e => e.SubmitterId)
                .OnDelete(DeleteBehavior.SetNull);

            entity.HasOne(e => e.Reviewer)
                .WithMany()
                .HasForeignKey(e => e.ReviewerId)
                .OnDelete(DeleteBehavior.SetNull);

            entity.HasIndex(e => e.Status);
            entity.HasIndex(e => new { e.Kind, e.Status });
            entity.HasIndex(e => new { e.TargetType, e.TargetId });
            entity.HasIndex(e => e.SubmitterId);

            // At most one *pending Edit* per user per target (ReviewTaskStatus.Pending = 0, ReviewTaskKind.Edit = 1).
            entity.HasIndex(e => new { e.SubmitterId, e.TargetType, e.TargetId })
                .IsUnique()
                .HasFilter("\"Status\" = 0 AND \"Kind\" = 1");
        });

        modelBuilder.Entity<DailyStat>(entity =>
        {
            entity.HasKey(e => e.Id);
            entity.HasIndex(e => e.Date).IsUnique();
        });

        modelBuilder.Entity<GlobalStat>(entity =>
        {
            entity.HasKey(e => e.Key);
        });
    }
}