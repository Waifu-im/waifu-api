using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Pgvector.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.UpdateImage;

public record UpdateImageCommand(
    long Id,
    string? Source,
    bool IsNsfw,
    long? UserId,
    List<string>? TagSlugs, // Slugs
    List<long>? ArtistIds,
    ReviewStatus? ReviewStatus,
    // Optional file replacement
    Stream? FileStream = null,
    string? FileName = null,
    string? ContentType = null
) : ICommand<ImageDto>;

public class UpdateImageCommandHandler : ICommandHandler<UpdateImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IStorageService _storageService;
    private readonly IImageProcessingService _imageProcessingService;
    private readonly string _cdnBaseUrl;
    private const int HammingDistanceThreshold = 4;

    public UpdateImageCommandHandler(
        IWaifuDbContext context,
        IStorageService storageService,
        IImageProcessingService imageProcessingService,
        IConfiguration configuration)
    {
        _context = context;
        _storageService = storageService;
        _imageProcessingService = imageProcessingService;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ImageDto> Handle(UpdateImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == request.Id, cancellationToken);

        if (image == null)
        {
            throw new KeyNotFoundException($"Image with ID {request.Id} not found.");
        }

        // Track old values for potential rollback and S3 cleanup
        var oldExtension = image.Extension;
        var oldPerceptualHash = image.PerceptualHash;
        var oldDominantColor = image.DominantColor;
        var oldIsAnimated = image.IsAnimated;
        var oldWidth = image.Width;
        var oldHeight = image.Height;
        var oldByteSize = image.ByteSize;
        var fileWasReplaced = false;
        string? newContentType = null;

        // Process new file if provided
        if (request.FileStream != null)
        {
            var metadata = await _imageProcessingService.ProcessAsync(request.FileStream, request.FileName!);

            // Check for duplicates (excluding the current image)
            var targetHash = BitArrayHelper.FromHex(metadata.PerceptualHash);
            var duplicate = await _context.Images
                .Where(i => i.Id != request.Id && i.PerceptualHash.HammingDistance(targetHash) <= HammingDistanceThreshold)
                .Select(i => new { i.Id })
                .FirstOrDefaultAsync(cancellationToken);

            if (duplicate != null)
            {
                throw new ConflictException($"Duplicate image found. ID: {duplicate.Id}");
            }

            // Update image metadata from new file
            image.PerceptualHash = targetHash;
            image.Extension = metadata.Extension;
            image.DominantColor = metadata.DominantColor;
            image.IsAnimated = metadata.IsAnimated;
            image.Width = metadata.Width;
            image.Height = metadata.Height;
            image.ByteSize = metadata.ByteSize;
            fileWasReplaced = true;
            newContentType = request.ContentType;
        }

        image.Source = request.Source.ToNullIfEmpty()?.Trim();
        image.IsNsfw = request.IsNsfw;

        // Allow setting UserId to null if 0 or null is passed, or update it if a valid ID is passed
        if (request.UserId.HasValue)
        {
            if (request.UserId.Value == 0)
            {
                image.UploaderId = null;
            }
            else
            {
                var user = await _context.Users.FindAsync(new object[] { request.UserId.Value }, cancellationToken);
                if (user == null)
                {
                    throw new KeyNotFoundException($"User with ID {request.UserId.Value} not found.");
                }
                image.UploaderId = request.UserId.Value;
            }
        }
        else
        {
            image.UploaderId = null;
        }

        if (request.TagSlugs != null)
        {
            image.Tags.Clear();
            if (request.TagSlugs.Any())
            {
                var trimmedSlugs = request.TagSlugs.Select(s => s.Trim()).ToList();
                var foundTags = await _context.Tags
                    .Where(t => trimmedSlugs.Contains(t.Slug))
                    .ToListAsync(cancellationToken);

                if (foundTags.Count != trimmedSlugs.Count)
                {
                    var foundSlugs = foundTags.Select(t => t.Slug).ToList();
                    var missingSlugs = trimmedSlugs.Except(foundSlugs).ToList();
                    throw new KeyNotFoundException($"Tags with slugs {string.Join(", ", missingSlugs)} not found.");
                }
                image.Tags = foundTags;
            }
        }

        if (request.ArtistIds != null)
        {
            image.Artists.Clear();
            if (request.ArtistIds.Any())
            {
                var foundArtists = await _context.Artists
                    .Where(a => request.ArtistIds.Contains(a.Id))
                    .ToListAsync(cancellationToken);

                if (foundArtists.Count != request.ArtistIds.Count)
                {
                    var foundIds = foundArtists.Select(a => a.Id).ToList();
                    var missingIds = request.ArtistIds.Except(foundIds).ToList();
                    throw new KeyNotFoundException($"Artists with IDs {string.Join(", ", missingIds)} not found.");
                }
                image.Artists = foundArtists;
            }
        }

        if (request.ReviewStatus.HasValue)
        {
            image.ReviewStatus = request.ReviewStatus.Value;
        }

        // Keep the review queue in sync if a moderator accepts directly rather than via the queue.
        if (request.ReviewStatus == ReviewStatus.Accepted)
        {
            var openTask = await _context.ReviewTasks.FirstOrDefaultAsync(t =>
                t.Kind == ReviewTaskKind.NewContent &&
                t.TargetType == ReviewableContentType.Image &&
                t.TargetId == image.Id &&
                t.Status == ReviewTaskStatus.Pending, cancellationToken);
            if (openTask != null)
            {
                openTask.Status = ReviewTaskStatus.Approved;
                openTask.ResolvedAt = DateTime.UtcNow;
            }
        }

        await _context.SaveChangesAsync(cancellationToken);

        // Handle S3 upload if file was replaced
        if (fileWasReplaced && request.FileStream != null)
        {
            try
            {
                request.FileStream.Position = 0;
                var newS3FileName = $"{image.Id}{image.Extension}";
                await _storageService.UploadAsync(request.FileStream, newS3FileName, newContentType!);

                // If extension changed, delete the old file from S3
                if (oldExtension != image.Extension)
                {
                    var oldS3FileName = $"{image.Id}{oldExtension}";
                    try
                    {
                        await _storageService.DeleteAsync(oldS3FileName);
                    }
                    catch
                    {
                        // Log but don't fail if old file deletion fails
                        // The old file will be orphaned but the new one is correctly uploaded
                    }
                }
            }
            catch
            {
                // Rollback: restore old metadata values
                image.PerceptualHash = oldPerceptualHash;
                image.Extension = oldExtension;
                image.DominantColor = oldDominantColor;
                image.IsAnimated = oldIsAnimated;
                image.Width = oldWidth;
                image.Height = oldHeight;
                image.ByteSize = oldByteSize;
                await _context.SaveChangesAsync(cancellationToken);
                throw;
            }
        }

        var dto = image.ToDto(_cdnBaseUrl, includeUploaderId: true);
        dto.Favorites = 0;
        dto.LikedAt = null;
        return dto;
    }
}