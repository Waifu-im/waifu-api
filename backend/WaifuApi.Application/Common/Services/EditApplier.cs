using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Artists.UpdateArtist;
using WaifuApi.Application.Features.Images.UpdateImage;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Services;

/// <inheritdoc />
public class EditApplier : IEditApplier
{
    private readonly IWaifuDbContext _context;
    private readonly IMediator _mediator;

    public EditApplier(IWaifuDbContext context, IMediator mediator)
    {
        _context = context;
        _mediator = mediator;
    }

    public Task ApplyAsync(ReviewTask task, CancellationToken cancellationToken) => task.TargetType switch
    {
        ReviewableContentType.Image => ApplyImageAsync(task, cancellationToken),
        ReviewableContentType.Tag => ApplyTagAsync(task, cancellationToken),
        ReviewableContentType.Artist => ApplyArtistAsync(task, cancellationToken),
        _ => throw new ArgumentOutOfRangeException(nameof(task))
    };

    private async Task ApplyImageAsync(ReviewTask task, CancellationToken ct)
    {
        var payload = Deserialize<ImageEditPayload>(task.Payload);

        var image = await _context.Images
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == task.TargetId, ct)
            ?? throw new KeyNotFoundException($"Image with ID {task.TargetId} not found.");

        // The Update command treats a null/absent field as "clear", so every value the payload does NOT
        // change must be passed through with its current value (especially the uploader and review status).
        // The proposed value simply overwrites whatever the field currently holds.
        var source = payload.Source != null ? payload.Source : image.Source;
        var isNsfw = payload.IsNsfw ?? image.IsNsfw;

        List<string>? tagSlugs = null;
        if (payload.AddTagSlugs is { Count: > 0 } || payload.RemoveTagSlugs is { Count: > 0 })
        {
            var slugs = image.Tags.Select(t => t.Slug).ToHashSet();
            foreach (var s in payload.AddTagSlugs ?? Enumerable.Empty<string>()) slugs.Add(s.Trim());
            foreach (var s in payload.RemoveTagSlugs ?? Enumerable.Empty<string>()) slugs.Remove(s.Trim());
            tagSlugs = slugs.ToList();
        }

        List<long>? artistIds = null;
        if (payload.AddArtistIds is { Count: > 0 } || payload.RemoveArtistIds is { Count: > 0 })
        {
            var ids = image.Artists.Select(a => a.Id).ToHashSet();
            foreach (var id in payload.AddArtistIds ?? Enumerable.Empty<long>()) ids.Add(id);
            foreach (var id in payload.RemoveArtistIds ?? Enumerable.Empty<long>()) ids.Remove(id);
            artistIds = ids.ToList();
        }

        await _mediator.Send(new UpdateImageCommand(
            image.Id,
            source,
            isNsfw,
            image.UploaderId, // preserve uploader
            tagSlugs,         // null => unchanged
            artistIds,        // null => unchanged
            null              // ReviewStatus => unchanged (stays Accepted)
        ), ct);
    }

    private async Task ApplyTagAsync(ReviewTask task, CancellationToken ct)
    {
        var payload = Deserialize<TagEditPayload>(task.Payload);

        var tag = await _context.Tags.FindAsync(new object[] { task.TargetId }, ct)
                  ?? throw new KeyNotFoundException($"Tag with ID {task.TargetId} not found.");

        await _mediator.Send(new UpdateTagCommand(
            tag.Id,
            payload.Name != null ? payload.Name.Trim() : tag.Name,
            payload.Description != null ? payload.Description.Trim() : tag.Description,
            null, // slug => unchanged (slugs are not editable via edits)
            null, // ReviewStatus => unchanged
            null  // CreatorId => unchanged
        ), ct);
    }

    private async Task ApplyArtistAsync(ReviewTask task, CancellationToken ct)
    {
        var payload = Deserialize<ArtistEditPayload>(task.Payload);

        var artist = await _context.Artists.FindAsync(new object[] { task.TargetId }, ct)
                     ?? throw new KeyNotFoundException($"Artist with ID {task.TargetId} not found.");

        await _mediator.Send(new UpdateArtistCommand(
            artist.Id,
            payload.Name != null ? payload.Name.Trim() : artist.Name,
            payload.Patreon != null ? payload.Patreon : artist.Patreon,
            payload.Pixiv != null ? payload.Pixiv : artist.Pixiv,
            payload.Twitter != null ? payload.Twitter : artist.Twitter,
            payload.DeviantArt != null ? payload.DeviantArt : artist.DeviantArt,
            null, // ReviewStatus => unchanged
            null  // CreatorId => unchanged
        ), ct);
    }

    private static T Deserialize<T>(string? json) where T : new()
        => json == null ? new T() : JsonSerializer.Deserialize<T>(json, EditPayloadOptions.Strict) ?? new T();
}
