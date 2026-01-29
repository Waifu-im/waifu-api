using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
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
    List<long>? ArtistIds
) : ICommand<ImageDto>;

public class UpdateImageCommandHandler : ICommandHandler<UpdateImageCommand, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public UpdateImageCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
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

        image.Source = request.Source.ToNullIfEmpty();
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
                var foundTags = await _context.Tags
                    .Where(t => request.TagSlugs.Contains(t.Slug))
                    .ToListAsync(cancellationToken);

                if (foundTags.Count != request.TagSlugs.Count)
                {
                    var foundSlugs = foundTags.Select(t => t.Slug).ToList();
                    var missingSlugs = request.TagSlugs.Except(foundSlugs).ToList();
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

        await _context.SaveChangesAsync(cancellationToken);

        return new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayHelper.ToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artists = image.Artists
                .Select(a => new ArtistDto 
                {
                    Id = a.Id,
                    Name = a.Name,
                    Patreon = a.Patreon,
                    Pixiv = a.Pixiv,
                    Twitter = a.Twitter,
                    DeviantArt = a.DeviantArt,
                    ReviewStatus = a.ReviewStatus
                }).ToList(),
            UploaderId = image.UploaderId ?? 0, // Return 0 if null for DTO compatibility if needed, or change DTO to nullable
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            Tags = image.Tags.Select(t => new TagDto
            {
                Id = t.Id,
                Name = t.Name,
                Slug = t.Slug,
                Description = t.Description,
                ReviewStatus = t.ReviewStatus
            }).ToList(),
            Favorites = 0, 
            LikedAt = null,
            ReviewStatus = image.ReviewStatus
        };
    }
}