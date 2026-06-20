using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Diagnostics;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using WaifuApi.Application;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Tests.Infrastructure;

/// <summary>
/// Wires the real Application services (Mediator handlers, validators, IReviewPolicy,
/// IEditRequestApplier) over an isolated EF InMemory database, with fakes for storage/image
/// processing. Each instance gets its own database so tests are independent.
/// </summary>
public sealed class TestHarness : IDisposable
{
    private readonly ServiceProvider _services;

    public TestHarness(Dictionary<string, string?>? configOverrides = null)
    {
        var config = new Dictionary<string, string?>
        {
            ["Cdn:BaseUrl"] = "https://cdn.test",
            ["Review:RequireEditRequestReview"] = "true",
            ["Review:RequireImageReview"] = "true",
            ["Review:RequireTagReview"] = "true",
            ["Review:RequireArtistReview"] = "true",
            ["EditRequest:DefaultPageSize"] = "30",
            ["EditRequest:MaxPageSize"] = "-1"
        };
        if (configOverrides != null)
        {
            foreach (var kv in configOverrides) config[kv.Key] = kv.Value;
        }

        var configuration = new ConfigurationBuilder().AddInMemoryCollection(config).Build();
        var dbName = Guid.NewGuid().ToString();

        var services = new ServiceCollection();
        services.AddSingleton<IConfiguration>(configuration);
        services.AddDbContext<TestDbContext>(o => o
            .UseInMemoryDatabase(dbName)
            .ConfigureWarnings(w => w.Ignore(InMemoryEventId.TransactionIgnoredWarning)));
        services.AddScoped<IWaifuDbContext>(sp => sp.GetRequiredService<TestDbContext>());
        services.AddScoped<IStorageService, FakeStorageService>();
        services.AddScoped<IImageProcessingService, FakeImageProcessingService>();
        services.AddApplication();

        _services = services.BuildServiceProvider();
    }

    public IServiceScope CreateScope() => _services.CreateScope();

    /// <summary>Send a command through the real Mediator pipeline in its own scope (like a single request).</summary>
    public async Task<TResponse> SendAsync<TResponse>(ICommand<TResponse> command)
    {
        using var scope = CreateScope();
        var mediator = scope.ServiceProvider.GetRequiredService<IMediator>();
        return await mediator.Send(command);
    }

    /// <summary>Mutate the database in a dedicated scope.</summary>
    public async Task WriteAsync(Func<TestDbContext, Task> action)
    {
        using var scope = CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<TestDbContext>();
        await action(db);
        await db.SaveChangesAsync(CancellationToken.None);
    }

    /// <summary>Read from the database in a fresh, no-tracking scope.</summary>
    public async Task<T> ReadAsync<T>(Func<TestDbContext, Task<T>> query)
    {
        using var scope = CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<TestDbContext>();
        return await query(db);
    }

    // ---- seeding helpers -------------------------------------------------

    public async Task<long> SeedUserAsync(Role role = Role.User)
    {
        var user = new User { Name = "user-" + Guid.NewGuid().ToString("N")[..8], DiscordId = Guid.NewGuid().ToString("N"), Role = role };
        await WriteAsync(db => { db.Users.Add(user); return Task.CompletedTask; });
        return user.Id;
    }

    public async Task<long> SeedTagAsync(string name, string description, ReviewStatus status = ReviewStatus.Accepted)
    {
        var tag = new Tag { Name = name, Slug = name.ToLowerInvariant().Replace(' ', '-'), Description = description, ReviewStatus = status };
        await WriteAsync(db => { db.Tags.Add(tag); return Task.CompletedTask; });
        return tag.Id;
    }

    public async Task<long> SeedArtistAsync(string name, ReviewStatus status = ReviewStatus.Accepted, string? twitter = null)
    {
        var artist = new Artist { Name = name, Twitter = twitter, ReviewStatus = status };
        await WriteAsync(db => { db.Artists.Add(artist); return Task.CompletedTask; });
        return artist.Id;
    }

    public async Task<long> SeedImageAsync(long? uploaderId = null, bool isNsfw = false, string? source = null, ReviewStatus status = ReviewStatus.Accepted, List<long>? tagIds = null, List<long>? artistIds = null)
    {
        using var scope = CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<TestDbContext>();

        var tags = tagIds is { Count: > 0 }
            ? await db.Tags.Where(t => tagIds.Contains(t.Id)).ToListAsync()
            : new List<Tag>();
        var artists = artistIds is { Count: > 0 }
            ? await db.Artists.Where(a => artistIds.Contains(a.Id)).ToListAsync()
            : new List<Artist>();

        var image = new Image
        {
            PerceptualHash = new System.Collections.BitArray(64),
            Extension = ".png",
            DominantColor = "#000000",
            Source = source,
            IsNsfw = isNsfw,
            UploaderId = uploaderId,
            ReviewStatus = status,
            Width = 100,
            Height = 100,
            ByteSize = 1000,
            Tags = tags,
            Artists = artists
        };

        db.Images.Add(image);
        await db.SaveChangesAsync(CancellationToken.None);
        return image.Id;
    }

    public async Task<long> SeedNewContentTaskAsync(ReviewableContentType type, long targetId, long? submitterId)
    {
        var task = new ReviewTask
        {
            Kind = ReviewTaskKind.NewContent,
            TargetType = type,
            TargetId = targetId,
            SubmitterId = submitterId,
            Status = ReviewTaskStatus.Pending,
            CreatedAt = DateTime.UtcNow
        };
        await WriteAsync(db => { db.ReviewTasks.Add(task); return Task.CompletedTask; });
        return task.Id;
    }

    public void Dispose() => _services.Dispose();
}
