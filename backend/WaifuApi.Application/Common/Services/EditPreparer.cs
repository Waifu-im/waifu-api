using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Services;

/// <inheritdoc />
public class EditPreparer : IEditPreparer
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public EditPreparer(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public Task<PreparedEdit> PrepareAsync(ReviewableContentType type, long targetId, string payloadJson, CancellationToken cancellationToken) => type switch
    {
        ReviewableContentType.Image => PrepareImageAsync(payloadJson, targetId, cancellationToken),
        ReviewableContentType.Tag => PrepareTagAsync(payloadJson, targetId, cancellationToken),
        ReviewableContentType.Artist => PrepareArtistAsync(payloadJson, targetId, cancellationToken),
        _ => throw new ArgumentException("Unsupported target type.")
    };

    private async Task<PreparedEdit> PrepareImageAsync(string rawPayload, long targetId, CancellationToken ct)
    {
        var payload = Deserialize<ImageEditPayload>(rawPayload);

        var image = await _context.Images
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == targetId && i.ReviewStatus == ReviewStatus.Accepted, ct)
            ?? throw new KeyNotFoundException($"Image with ID {targetId} not found.");

        var proposedSource = payload.Source?.Trim();
        var hasSource = !string.IsNullOrEmpty(proposedSource);
        if (hasSource)
        {
            if (proposedSource!.Length > 500) throw new ArgumentException("Source URL must not exceed 500 characters.");
            if (!IsValidWebUrl(proposedSource)) throw new ArgumentException("Source must be a valid http(s) URL.");
        }

        var addTags = Clean(payload.AddTagSlugs);
        var removeTags = Clean(payload.RemoveTagSlugs);
        var addArtists = (payload.AddArtistIds ?? new List<long>()).Where(id => id > 0).Distinct().ToList();
        var removeArtists = (payload.RemoveArtistIds ?? new List<long>()).Where(id => id > 0).Distinct().ToList();

        if (addTags.Count > 0)
        {
            var found = await _context.Tags.Where(t => addTags.Contains(t.Slug)).Select(t => t.Slug).ToListAsync(ct);
            var missing = addTags.Except(found).ToList();
            if (missing.Count > 0) throw new KeyNotFoundException($"Tags with slugs {string.Join(", ", missing)} not found.");
        }
        if (addArtists.Count > 0)
        {
            var found = await _context.Artists.Where(a => addArtists.Contains(a.Id)).Select(a => a.Id).ToListAsync(ct);
            var missing = addArtists.Except(found).ToList();
            if (missing.Count > 0) throw new KeyNotFoundException($"Artists with IDs {string.Join(", ", missing)} not found.");
        }

        var currentSlugs = image.Tags.Select(t => t.Slug).ToHashSet();
        var currentArtistIds = image.Artists.Select(a => a.Id).ToHashSet();

        var effectiveAddTags = addTags.Where(s => !currentSlugs.Contains(s)).ToList();
        var effectiveRemoveTags = removeTags.Where(s => currentSlugs.Contains(s)).ToList();
        var effectiveAddArtists = addArtists.Where(id => !currentArtistIds.Contains(id)).ToList();
        var effectiveRemoveArtists = removeArtists.Where(id => currentArtistIds.Contains(id)).ToList();

        var sourceChanges = hasSource && proposedSource != image.Source;
        var nsfwChanges = payload.IsNsfw.HasValue && payload.IsNsfw.Value != image.IsNsfw;

        if (!sourceChanges && !nsfwChanges &&
            effectiveAddTags.Count == 0 && effectiveRemoveTags.Count == 0 &&
            effectiveAddArtists.Count == 0 && effectiveRemoveArtists.Count == 0)
        {
            throw new ArgumentException("The proposed changes are empty or identical to the current values.");
        }

        var normalized = new ImageEditPayload
        {
            Source = sourceChanges ? proposedSource : null,
            IsNsfw = nsfwChanges ? payload.IsNsfw : null,
            AddTagSlugs = effectiveAddTags.Count > 0 ? effectiveAddTags : null,
            RemoveTagSlugs = effectiveRemoveTags.Count > 0 ? effectiveRemoveTags : null,
            AddArtistIds = effectiveAddArtists.Count > 0 ? effectiveAddArtists : null,
            RemoveArtistIds = effectiveRemoveArtists.Count > 0 ? effectiveRemoveArtists : null
        };

        var target = new ReviewTaskTargetDto { Image = image.ToDto(_cdnBaseUrl, includeUploaderId: true) };
        return new PreparedEdit(Serialize(normalized), target);
    }

    private async Task<PreparedEdit> PrepareTagAsync(string rawPayload, long targetId, CancellationToken ct)
    {
        var payload = Deserialize<TagEditPayload>(rawPayload);

        var tag = await _context.Tags
            .FirstOrDefaultAsync(t => t.Id == targetId && t.ReviewStatus == ReviewStatus.Accepted, ct)
            ?? throw new KeyNotFoundException($"Tag with ID {targetId} not found.");

        var proposedName = payload.Name?.Trim();
        var hasName = !string.IsNullOrEmpty(proposedName);
        if (hasName && proposedName!.Length > 30) throw new ArgumentException("Name must not exceed 30 characters.");

        var proposedDescription = payload.Description?.Trim();
        var hasDescription = !string.IsNullOrEmpty(proposedDescription);
        if (hasDescription && proposedDescription!.Length > 150) throw new ArgumentException("Description must not exceed 150 characters.");

        var nameChanges = hasName && proposedName != tag.Name;
        var descriptionChanges = hasDescription && proposedDescription != tag.Description;

        if (!nameChanges && !descriptionChanges)
        {
            throw new ArgumentException("The proposed changes are empty or identical to the current values.");
        }

        var normalized = new TagEditPayload
        {
            Name = nameChanges ? proposedName : null,
            Description = descriptionChanges ? proposedDescription : null
        };

        var target = new ReviewTaskTargetDto { Tag = tag.ToDto() };
        return new PreparedEdit(Serialize(normalized), target);
    }

    private async Task<PreparedEdit> PrepareArtistAsync(string rawPayload, long targetId, CancellationToken ct)
    {
        var payload = Deserialize<ArtistEditPayload>(rawPayload);

        var artist = await _context.Artists
            .FirstOrDefaultAsync(a => a.Id == targetId && a.ReviewStatus == ReviewStatus.Accepted, ct)
            ?? throw new KeyNotFoundException($"Artist with ID {targetId} not found.");

        var proposedName = payload.Name?.Trim();
        var hasName = !string.IsNullOrEmpty(proposedName);
        if (hasName && proposedName!.Length > 30) throw new ArgumentException("Name must not exceed 30 characters.");
        var nameChanges = hasName && proposedName != artist.Name;

        var patreon = NormalizeLink(payload.Patreon, "Patreon");
        var pixiv = NormalizeLink(payload.Pixiv, "Pixiv");
        var twitter = NormalizeLink(payload.Twitter, "Twitter");
        var deviantArt = NormalizeLink(payload.DeviantArt, "DeviantArt");

        var patreonChanges = patreon != null && patreon != artist.Patreon;
        var pixivChanges = pixiv != null && pixiv != artist.Pixiv;
        var twitterChanges = twitter != null && twitter != artist.Twitter;
        var deviantArtChanges = deviantArt != null && deviantArt != artist.DeviantArt;

        if (!nameChanges && !patreonChanges && !pixivChanges && !twitterChanges && !deviantArtChanges)
        {
            throw new ArgumentException("The proposed changes are empty or identical to the current values.");
        }

        var normalized = new ArtistEditPayload
        {
            Name = nameChanges ? proposedName : null,
            Patreon = patreonChanges ? patreon : null,
            Pixiv = pixivChanges ? pixiv : null,
            Twitter = twitterChanges ? twitter : null,
            DeviantArt = deviantArtChanges ? deviantArt : null
        };

        var target = new ReviewTaskTargetDto { Artist = artist.ToDto() };
        return new PreparedEdit(Serialize(normalized), target);
    }

    private static string? NormalizeLink(string? value, string fieldName)
    {
        var trimmed = value?.Trim();
        if (string.IsNullOrEmpty(trimmed)) return null;
        if (trimmed.Length > 200) throw new ArgumentException($"{fieldName} link must not exceed 200 characters.");
        if (!IsValidWebUrl(trimmed)) throw new ArgumentException($"{fieldName} link must be a valid http(s) URL.");
        return trimmed;
    }

    private static List<string> Clean(List<string>? slugs) =>
        (slugs ?? new List<string>())
        .Select(s => s?.Trim() ?? string.Empty)
        .Where(s => s.Length > 0)
        .Distinct()
        .ToList();

    private static bool IsValidWebUrl(string url) =>
        Uri.TryCreate(url, UriKind.Absolute, out var uri) &&
        (uri.Scheme == Uri.UriSchemeHttp || uri.Scheme == Uri.UriSchemeHttps);

    private static T Deserialize<T>(string json) where T : new()
    {
        try
        {
            return JsonSerializer.Deserialize<T>(json, EditPayloadOptions.Strict) ?? new T();
        }
        catch (JsonException ex)
        {
            throw new ArgumentException($"Invalid payload: {ex.Message}");
        }
    }

    private static string Serialize(object value) => JsonSerializer.Serialize(value, EditPayloadOptions.Strict);
}
